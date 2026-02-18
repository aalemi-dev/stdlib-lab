package metrics_test

import (
	"testing"

	"github.com/aalemi-dev/stdlib-lab/logger"
	"github.com/aalemi-dev/stdlib-lab/metrics"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestFXModule_ProvidesMetrics(t *testing.T) {
	t.Parallel()
	var m *metrics.Metrics

	app := fxtest.New(t,
		metrics.FXModule,
		fx.Provide(func() metrics.Config {
			empty := ""
			port := ":0"
			return metrics.Config{
				ServiceName:               "fx-test",
				SystemMetricsAddress:      &empty,
				ApplicationMetricsAddress: &port,
			}
		}),
		fx.Provide(func() *logger.LoggerClient {
			return logger.NewLoggerClient(logger.Config{Level: logger.Info})
		}),
		fx.Populate(&m),
	)

	app.RequireStart()
	defer app.RequireStop()

	if m == nil {
		t.Fatal("expected non-nil *Metrics")
	}
}

func TestFXModule_ProvidesCollectorInterface(t *testing.T) {
	t.Parallel()
	var collector metrics.MetricsCollector

	app := fxtest.New(t,
		metrics.FXModule,
		fx.Provide(func() metrics.Config {
			empty := ""
			port := ":0"
			return metrics.Config{
				ServiceName:               "fx-test",
				SystemMetricsAddress:      &empty,
				ApplicationMetricsAddress: &port,
			}
		}),
		fx.Provide(func() *logger.LoggerClient {
			return logger.NewLoggerClient(logger.Config{Level: logger.Info})
		}),
		fx.Populate(&collector),
	)

	app.RequireStart()
	defer app.RequireStop()

	if collector == nil {
		t.Fatal("expected non-nil MetricsCollector")
	}
}

func TestRegisterMetricsLifecycle_StartsAndStops(t *testing.T) {
	t.Parallel()
	empty := ""
	port := ":0"

	m := metrics.NewMetrics(metrics.Config{
		ServiceName:               "lifecycle-test",
		SystemMetricsAddress:      &port,
		ApplicationMetricsAddress: &empty,
	})

	log := logger.NewLoggerClient(logger.Config{Level: logger.Info})

	app := fxtest.New(t,
		fx.Provide(func() *metrics.Metrics { return m }),
		fx.Provide(func() *logger.LoggerClient { return log }),
		fx.Invoke(metrics.RegisterMetricsLifecycle),
	)

	app.RequireStart()
	app.RequireStop()
}

func TestRegisterMetricsLifecycle_BothServersStarted(t *testing.T) {
	t.Parallel()
	sysPort := ":0"
	appPort := ":0"

	m := metrics.NewMetrics(metrics.Config{
		ServiceName:               "both-servers-test",
		SystemMetricsAddress:      &sysPort,
		ApplicationMetricsAddress: &appPort,
	})

	log := logger.NewLoggerClient(logger.Config{Level: logger.Info})

	app := fxtest.New(t,
		fx.Provide(func() *metrics.Metrics { return m }),
		fx.Provide(func() *logger.LoggerClient { return log }),
		fx.Invoke(metrics.RegisterMetricsLifecycle),
	)

	app.RequireStart()
	app.RequireStop()
}

func TestRegisterMetricsLifecycle_BothServersNil(t *testing.T) {
	t.Parallel()
	empty := ""

	m := metrics.NewMetrics(metrics.Config{
		ServiceName:               "nil-servers-test",
		SystemMetricsAddress:      &empty,
		ApplicationMetricsAddress: &empty,
	})

	log := logger.NewLoggerClient(logger.Config{Level: logger.Info})

	app := fxtest.New(t,
		fx.Provide(func() *metrics.Metrics { return m }),
		fx.Provide(func() *logger.LoggerClient { return log }),
		fx.Invoke(metrics.RegisterMetricsLifecycle),
	)

	app.RequireStart()
	app.RequireStop()
}
