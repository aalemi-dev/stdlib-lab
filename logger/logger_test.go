package logger

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newObservedLogger creates a LoggerClient backed by an in-memory observer
// so tests can assert on emitted log entries without writing to stderr.
func newObservedLogger(level zapcore.Level, tracingEnabled bool) (*LoggerClient, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return &LoggerClient{
		Zap:            zap.New(core),
		tracingEnabled: tracingEnabled,
	}, logs
}

// --- NewLoggerClient ---

func TestNewLoggerClient_Levels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level    string
		expected zapcore.Level
	}{
		{Debug, zapcore.DebugLevel},
		{Info, zapcore.InfoLevel},
		{Warning, zapcore.WarnLevel},
		{Error, zapcore.ErrorLevel},
		{"unknown", zapcore.InfoLevel}, // defaults to info
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			t.Parallel()
			l := NewLoggerClient(Config{Level: tc.level, ServiceName: "test"})
			if l == nil {
				t.Fatal("expected non-nil LoggerClient")
			}
			if l.Zap == nil {
				t.Fatal("expected non-nil Zap logger")
			}
		})
	}
}

func TestNewLoggerClient_TracingEnabled(t *testing.T) {
	t.Parallel()
	l := NewLoggerClient(Config{Level: Info, EnableTracing: true})
	if !l.tracingEnabled {
		t.Error("expected tracingEnabled to be true")
	}
}

func TestNewLoggerClient_DefaultCallerSkip(t *testing.T) {
	t.Parallel()
	// CallerSkip <= 0 should not panic; it defaults to 1 internally
	l := NewLoggerClient(Config{Level: Info, CallerSkip: 0})
	if l == nil {
		t.Fatal("expected non-nil LoggerClient")
	}
}

// --- convertToZapFields ---

func TestConvertToZapFields_NilError(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, false)
	fields := l.convertToZapFields(nil)
	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(fields))
	}
}

func TestConvertToZapFields_WithError(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, false)
	err := errors.New("something went wrong")
	fields := l.convertToZapFields(err)
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "error" {
		t.Errorf("expected key 'error', got %q", fields[0].Key)
	}
}

func TestConvertToZapFields_WithFieldMaps(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, false)
	fields := l.convertToZapFields(nil,
		map[string]interface{}{"key1": "val1"},
		map[string]interface{}{"key2": 42},
	)
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestConvertToZapFields_ErrorAndFields(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, false)
	err := errors.New("oops")
	fields := l.convertToZapFields(err, map[string]interface{}{"k": "v"})
	if len(fields) != 2 {
		t.Errorf("expected 2 fields (error + k), got %d", len(fields))
	}
}

// --- Basic logging methods ---

func TestInfo(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.InfoLevel, false)
	l.Info("hello", nil, map[string]interface{}{"k": "v"})

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Message != "hello" {
		t.Errorf("expected message 'hello', got %q", entry.Message)
	}
	if entry.Level != zapcore.InfoLevel {
		t.Errorf("expected INFO level, got %v", entry.Level)
	}
}

func TestDebug(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.DebugLevel, false)
	l.Debug("debug msg", nil)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Level != zapcore.DebugLevel {
		t.Errorf("expected DEBUG level")
	}
}

func TestDebug_SuppressedAtInfoLevel(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.InfoLevel, false)
	l.Debug("should not appear", nil)
	if logs.Len() != 0 {
		t.Errorf("expected debug entry to be suppressed, got %d entries", logs.Len())
	}
}

func TestWarn(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.WarnLevel, false)
	l.Warn("warn msg", nil)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Level != zapcore.WarnLevel {
		t.Errorf("expected WARN level")
	}
}

func TestError(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.ErrorLevel, false)
	err := errors.New("boom")
	l.Error("error msg", err)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("expected ERROR level")
	}
	if entry.ContextMap()["error"] != "boom" {
		t.Errorf("expected error field to be 'boom'")
	}
}

// --- Context-aware logging methods ---

func TestInfoWithContext_NoSpan(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.InfoLevel, true)
	l.InfoWithContext(context.Background(), "ctx info", nil)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	// No active span — trace fields must not be present
	fields := logs.All()[0].ContextMap()
	if _, ok := fields["trace_id"]; ok {
		t.Error("did not expect trace_id without an active span")
	}
}

func TestDebugWithContext(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.DebugLevel, false)
	l.DebugWithContext(context.Background(), "ctx debug", nil)
	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Level != zapcore.DebugLevel {
		t.Errorf("expected DEBUG level")
	}
}

func TestWarnWithContext(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.WarnLevel, false)
	l.WarnWithContext(context.Background(), "ctx warn", nil)
	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Level != zapcore.WarnLevel {
		t.Errorf("expected WARN level")
	}
}

func TestErrorWithContext(t *testing.T) {
	t.Parallel()
	l, logs := newObservedLogger(zapcore.ErrorLevel, false)
	err := errors.New("ctx error")
	l.ErrorWithContext(context.Background(), "ctx error msg", err)
	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Level != zapcore.ErrorLevel {
		t.Errorf("expected ERROR level")
	}
}

// --- extractTracingFields ---

func TestExtractTracingFields_TracingDisabled(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, false)
	fields := l.extractTracingFields(context.Background())
	if len(fields) != 0 {
		t.Errorf("expected no fields when tracing is disabled, got %d", len(fields))
	}
}

func TestExtractTracingFields_NilContext(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, true)
	//nolint:staticcheck // intentionally passing nil to test guard
	fields := l.extractTracingFields(nil)
	if len(fields) != 0 {
		t.Errorf("expected no fields for nil context, got %d", len(fields))
	}
}

func TestExtractTracingFields_NoActiveSpan(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.DebugLevel, true)
	// context.Background() has no span — should return nil
	fields := l.extractTracingFields(context.Background())
	if len(fields) != 0 {
		t.Errorf("expected no fields without an active span, got %d", len(fields))
	}
}

// --- Logger interface compliance ---

func TestLoggerClient_ImplementsLogger(t *testing.T) {
	t.Parallel()
	l, _ := newObservedLogger(zapcore.InfoLevel, false)
	var _ Logger = l // compile-time check
}
