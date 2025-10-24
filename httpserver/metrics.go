package httpserver

import (
	"time"

	"github.com/bhmj/goblocks/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type serviceMetrics struct {
	errorsCounter *prometheus.CounterVec
	latency       *prometheus.HistogramVec
}

func newMetrics(metricsRegistry prometheus.Registerer, conf metrics.Config) *serviceMetrics {
	metrics := &serviceMetrics{}
	factory := promauto.With(metricsRegistry)

	defaultBuckets := []float64{
		0.002, 0.004, 0.006, 0.008, 0.010, 0.020, 0.050, 0.100, 0.200, 0.300, 0.500, 0.700, 0.900, 1.100, 1.300, 1.500,
	}
	var buckets []float64
	if len(conf.Buckets) > 0 {
		buckets = conf.Buckets
	} else {
		buckets = defaultBuckets
	}

	metrics.errorsCounter = factory.NewCounterVec(prometheus.CounterOpts{ //nolint:promlinter
		Name: "error_count",
		Help: "error count per method",
	}, []string{"service", "endpoint"})
	metrics.latency = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_latency",
		Help:    "total duration of request in seconds",
		Buckets: buckets,
	}, []string{"service", "endpoint"})

	return metrics
}

func (m *serviceMetrics) ScoreMethod(service, endpoint string, begin time.Time, err error) {
	labels := prometheus.Labels{
		"service":  service,
		"endpoint": endpoint,
	}
	if isError(err) {
		m.errorsCounter.With(labels).Add(1)
	}
	m.latency.With(labels).Observe(time.Since(begin).Seconds())
}

func isError(err error) bool {
	return err != nil
}
