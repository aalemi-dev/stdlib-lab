package tracer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_NoExport(t *testing.T) {
	t.Parallel()
	cfg := Config{
		ServiceName:  "test-service",
		AppEnv:       "test",
		EnableExport: false,
	}

	client, err := NewClient(cfg)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.tracer)
}

func TestNewClient_EmptyServiceName(t *testing.T) {
	t.Parallel()
	cfg := Config{
		ServiceName:  "",
		AppEnv:       "test",
		EnableExport: false,
	}

	client, err := NewClient(cfg)

	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_EnableExport_NoCollector(t *testing.T) {
	t.Parallel()
	cfg := Config{
		ServiceName:  "test-service",
		AppEnv:       "production",
		EnableExport: true,
	}

	// The OTLP HTTP exporter connects lazily, so NewClient succeeds even without a collector.
	// Spans will fail to export at flush time, but initialization itself is non-blocking.
	client, err := NewClient(cfg)

	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_EnableExport_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the exporter handshake fails

	cfg := Config{
		ServiceName:  "test-service",
		AppEnv:       "test",
		EnableExport: true,
	}

	client, err := newClientWithContext(ctx, cfg)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to initialize OTLP exporter")
}
