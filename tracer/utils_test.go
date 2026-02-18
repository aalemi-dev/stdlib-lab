package tracer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func newTestClient(t *testing.T) *TracerClient {
	t.Helper()
	client, err := NewClient(Config{ServiceName: "test", AppEnv: "test", EnableExport: false})
	require.NoError(t, err)
	return client
}

func TestStartSpan_ReturnsSpanAndContext(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	ctx, span := client.StartSpan(context.Background(), "test-op")

	assert.NotNil(t, span)
	assert.NotNil(t, ctx)
	span.End()
}

func TestStartSpan_SpanIsRecording(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	ctx, span := client.StartSpan(context.Background(), "test-op")
	defer span.End()

	otSpan := trace.SpanFromContext(ctx)
	assert.True(t, otSpan.IsRecording())
}

func TestStartSpan_ChildInheritsParent(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	parentCtx, parentSpan := client.StartSpan(context.Background(), "parent")
	defer parentSpan.End()

	childCtx, childSpan := client.StartSpan(parentCtx, "child")
	defer childSpan.End()

	parentOT := trace.SpanFromContext(parentCtx)
	childOT := trace.SpanFromContext(childCtx)

	assert.Equal(t, parentOT.SpanContext().TraceID(), childOT.SpanContext().TraceID())
}

func TestSpanEnd_DoesNotPanic(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	_, span := client.StartSpan(context.Background(), "test-op")

	assert.NotPanics(t, func() { span.End() })
}

func TestSetAttributes_AllTypes(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	_, span := client.StartSpan(context.Background(), "attrs-op")
	defer span.End()

	assert.NotPanics(t, func() {
		span.SetAttributes(map[string]interface{}{
			"str":     "hello",
			"int":     42,
			"int64":   int64(100),
			"float64": 3.14,
			"bool":    true,
			"other":   []string{"a", "b"}, // fallback to fmt.Sprint
		})
	})
}

func TestSetAttributes_EmptyMap(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	_, span := client.StartSpan(context.Background(), "attrs-op")
	defer span.End()

	assert.NotPanics(t, func() {
		span.SetAttributes(map[string]interface{}{})
	})
}

func TestRecordError(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	_, span := client.StartSpan(context.Background(), "err-op")
	defer span.End()

	assert.NotPanics(t, func() {
		span.RecordError(errors.New("something went wrong"))
	})
}

func TestGetCarrier_NoActiveSpan(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	carrier := client.GetCarrier(context.Background())

	// Without an active span the carrier has no traceparent.
	assert.NotNil(t, carrier)
}

func TestGetCarrier_WithActiveSpan(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	ctx, span := client.StartSpan(context.Background(), "carrier-op")
	defer span.End()

	carrier := client.GetCarrier(ctx)

	assert.NotEmpty(t, carrier)
	assert.Contains(t, carrier, "traceparent")
}

func TestSetCarrierOnContext_InjectsTrace(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	ctx, span := client.StartSpan(context.Background(), "inject-op")
	defer span.End()

	carrier := client.GetCarrier(ctx)

	newCtx := client.SetCarrierOnContext(context.Background(), carrier)

	injectedSpan := trace.SpanFromContext(newCtx)
	assert.Equal(t, span.(*spanImpl).span.SpanContext().TraceID(), injectedSpan.SpanContext().TraceID())
}

func TestSetCarrierOnContext_EmptyCarrier(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	newCtx := client.SetCarrierOnContext(context.Background(), map[string]string{})

	assert.NotNil(t, newCtx)
}

func TestGetAndSetCarrier_RoundTrip(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	ctx, span := client.StartSpan(context.Background(), "roundtrip-op")
	defer span.End()

	carrier := client.GetCarrier(ctx)
	restoredCtx := client.SetCarrierOnContext(context.Background(), carrier)

	original := trace.SpanFromContext(ctx).SpanContext()
	restored := trace.SpanFromContext(restoredCtx).SpanContext()

	assert.Equal(t, original.TraceID(), restored.TraceID())
	assert.True(t, restored.IsValid())
}
