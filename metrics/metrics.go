package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Namespace string    `yaml:"namespace" description:"Metrics namespace" required:"true"`
	Buckets   []float64 `yaml:"buckets" description:"List of buckets for request latency histogram metric"`
}

type Registry struct {
	registry *prometheus.Registry
	prefix   string
}

func NewRegistry(config Config) (*Registry, error) {
	registry := &Registry{
		registry: prometheus.NewRegistry(),
		prefix:   config.Namespace + "_",
	}

	// Default system metrics
	if err := registry.Get().Register(collectors.NewGoCollector()); err != nil {
		return nil, fmt.Errorf("failed to register go collector: %w", err)
	}
	if err := registry.Get().Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("failed to register process collector: %w", err)
	}

	return registry, nil
}

func (r *Registry) Handler() http.Handler {
	return promhttp.InstrumentMetricHandler(
		r.Get(), promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{}),
	)
}

func (r *Registry) Get() prometheus.Registerer {
	return prometheus.WrapRegistererWithPrefix(r.prefix, r.registry)
}
