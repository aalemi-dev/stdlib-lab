package logger_test

import (
	"context"
	"errors"

	"github.com/aalemi-dev/stdlib-lab/logger"
)

func ExampleNewLoggerClient() {
	log := logger.NewLoggerClient(logger.Config{
		Level:       logger.Info,
		ServiceName: "example-service",
	})

	log.Info("service started", nil)
}

func ExampleLoggerClient_Info() {
	log := logger.NewLoggerClient(logger.Config{
		Level:       logger.Info,
		ServiceName: "example-service",
	})

	log.Info("user logged in", nil, map[string]interface{}{
		"user_id": "12345",
		"ip":      "192.168.1.1",
	})
}

func ExampleLoggerClient_Error() {
	log := logger.NewLoggerClient(logger.Config{
		Level:       logger.Info,
		ServiceName: "example-service",
	})

	err := errors.New("connection refused")
	log.Error("database connection failed", err, map[string]interface{}{
		"host":        "localhost:5432",
		"retry_count": 3,
	})
}

func ExampleLoggerClient_Debug() {
	log := logger.NewLoggerClient(logger.Config{
		Level:       logger.Debug,
		ServiceName: "example-service",
	})

	log.Debug("processing request", nil, map[string]interface{}{
		"request_id":   "abc-123",
		"payload_size": 1024,
	})
}

func ExampleLoggerClient_InfoWithContext() {
	log := logger.NewLoggerClient(logger.Config{
		Level:         logger.Info,
		ServiceName:   "example-service",
		EnableTracing: true,
	})

	ctx := context.Background()

	// When an active OpenTelemetry span is present in ctx,
	// trace_id and span_id are automatically attached to the log entry.
	log.InfoWithContext(ctx, "handling request", nil, map[string]interface{}{
		"request_id": "abc-123",
	})
}

func ExampleLoggerClient_ErrorWithContext() {
	log := logger.NewLoggerClient(logger.Config{
		Level:         logger.Info,
		ServiceName:   "example-service",
		EnableTracing: true,
	})

	ctx := context.Background()
	err := errors.New("timeout")

	log.ErrorWithContext(ctx, "upstream call failed", err, map[string]interface{}{
		"service": "payments",
	})
}

func Example_callerSkip() {
	// When wrapping the logger in your own type, increase CallerSkip
	// so the reported caller points to your business logic, not the wrapper.
	log := logger.NewLoggerClient(logger.Config{
		Level:       logger.Info,
		ServiceName: "example-service",
		CallerSkip:  2,
	})

	log.Info("called from wrapper", nil)
}
