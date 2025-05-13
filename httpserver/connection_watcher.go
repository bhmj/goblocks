package httpserver

import (
	"net"
	"net/http"
	"sync/atomic"

	"github.com/bhmj/goblocks/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ConnectionWatcher struct {
	currentlyOpen             int64
	logger                    log.MetaLogger
	incomingConnectionsGauge  prometheus.Gauge
	incomingConnectionsOpened prometheus.Counter
	incomingConnectionsClosed prometheus.Counter
}

func NewConnectionWatcher(metricsRegistry prometheus.Registerer, logger log.MetaLogger) *ConnectionWatcher {
	factory := promauto.With(prometheus.WrapRegistererWithPrefix("httpserver_", metricsRegistry))
	return &ConnectionWatcher{
		logger: logger,
		incomingConnectionsGauge: factory.NewGauge(prometheus.GaugeOpts{
			Name: "incoming_connections",
			Help: "Open incoming connection gauge",
		}),
		incomingConnectionsOpened: factory.NewCounter(prometheus.CounterOpts{
			Name: "incoming_connections_opened_total",
			Help: "Incoming connections opened counter",
		}),
		incomingConnectionsClosed: factory.NewCounter(prometheus.CounterOpts{
			Name: "incoming_connections_closed_total",
			Help: "Incoming connections closed counter",
		}),
	}
}

// OnStateChange records open connections in response to connection state changes
func (cw *ConnectionWatcher) OnStateChange(conn net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		cw.incomingConnectionsGauge.Set(float64((atomic.AddInt64(&cw.currentlyOpen, 1))))
		cw.incomingConnectionsOpened.Inc()
		cw.logger.Debug("new incoming connection opened from " + conn.RemoteAddr().String())
	case http.StateHijacked, http.StateClosed:
		cw.incomingConnectionsGauge.Set(float64(atomic.AddInt64(&cw.currentlyOpen, -1)))
		cw.incomingConnectionsClosed.Inc()
		cw.logger.Debug("incoming connection closed from " + conn.RemoteAddr().String())
	case http.StateActive, http.StateIdle:
	}
}

// Count returns the current number of open connections
func (cw *ConnectionWatcher) Count() int64 {
	return atomic.LoadInt64(&cw.currentlyOpen)
}
