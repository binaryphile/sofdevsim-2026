package telemetry

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config controls telemetry export.
// Prometheus is enabled by default when Setup is called.
// OTLP/HTTP is enabled when OTLPAddr is non-empty.
type Config struct {
	Prometheus bool   // expose /metrics endpoint (default true)
	OTLPAddr   string // if non-empty, push via OTLP/HTTP to this address
}

// Telemetry holds the initialized telemetry components.
type Telemetry struct {
	Exporter    *Exporter
	PromHandler http.Handler // nil if Prometheus not enabled
	Shutdown    func(context.Context) error
}

// Setup initializes OTel meter provider with configured exporters.
// Returns Telemetry with exporter, optional Prometheus handler, and shutdown function.
func Setup(ctx context.Context, cfg Config) (*Telemetry, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("sofdevsim"),
			semconv.ServiceVersion("0.1.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	opts := []metric.Option{metric.WithResource(res)}
	var promHandler http.Handler

	// Prometheus reader (pull-based)
	if cfg.Prometheus {
		promExp, err := promexporter.New()
		if err != nil {
			return nil, err
		}
		opts = append(opts, metric.WithReader(promExp))
		promHandler = promhttp.Handler()
	}

	// Create meter provider with all readers
	provider := metric.NewMeterProvider(opts...)
	meter := provider.Meter("sofdevsim")

	exporter, err := NewExporter(meter)
	if err != nil {
		provider.Shutdown(ctx)
		return nil, err
	}

	return &Telemetry{
		Exporter:    exporter,
		PromHandler: promHandler,
		Shutdown: func(ctx context.Context) error {
			return provider.Shutdown(ctx)
		},
	}, nil
}
