package tracer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestFXModule_ProvidesTracerClient(t *testing.T) {
	t.Parallel()
	var client *TracerClient

	app := fxtest.New(t,
		FXModule,
		fx.Provide(func() Config {
			return Config{ServiceName: "fx-test", AppEnv: "test", EnableExport: false}
		}),
		fx.Populate(&client),
	)

	app.RequireStart()
	defer app.RequireStop()

	assert.NotNil(t, client)
}

func TestFXModule_ProvidesTracerInterface(t *testing.T) {
	t.Parallel()
	var tr Tracer

	app := fxtest.New(t,
		FXModule,
		fx.Provide(func() Config {
			return Config{ServiceName: "fx-test", AppEnv: "test", EnableExport: false}
		}),
		fx.Populate(&tr),
	)

	app.RequireStart()
	defer app.RequireStop()

	assert.NotNil(t, tr)
}

func TestRegisterTracerLifecycle_Shutdown(t *testing.T) {
	t.Parallel()
	client, err := NewClient(Config{ServiceName: "shutdown-test", AppEnv: "test", EnableExport: false})
	require.NoError(t, err)

	app := fxtest.New(t,
		fx.Provide(func() *TracerClient { return client }),
		fx.Invoke(RegisterTracerLifecycle),
	)

	app.RequireStart()
	assert.NotPanics(t, func() { app.RequireStop() })
}

func TestRegisterTracerLifecycle_NilTracer(t *testing.T) {
	t.Parallel()
	client := &TracerClient{tracer: nil}

	app := fxtest.New(t,
		fx.Provide(func() *TracerClient { return client }),
		fx.Invoke(RegisterTracerLifecycle),
	)

	app.RequireStart()
	assert.NotPanics(t, func() { app.RequireStop() })
}
