package conncount

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bhmj/goblocks/log"
)

// Transport defines a transport with build-in connection counter
// Usage:
// func callback(n int64) { x.metrics.ConnectionGauge(n) }
// transport := conncount.NewTransport(logger, logFields, &http.Transport{}, callback)
// httpClient := &httpClient{Transport: transport}
type Transport struct {
	*http.Transport
	connCounter *int64
}

type dialer func(ctx context.Context, network string, addr string) (net.Conn, error)

// NewTransport creates Transport with a connection counter.
// prev is a Transport to be wrapped
// callback is a function to be called when connection counter changes
func NewTransport(logger log.MetaLogger, prev *http.Transport, callback func(int64)) *Transport {
	var counter int64
	tran := &Transport{Transport: prev, connCounter: &counter}
	prevDialer := tran.getPreviousDialer()
	prevTLSDialer := tran.getPreviousTLSDialer()
	dialWithCounter := func(prev dialer) dialer {
		if prev == nil {
			return nil
		}
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			begin := time.Now()
			conn, err := prev(ctx, network, addr)
			if err != nil {
				logger.Error("connection open", log.String("latency", time.Since(begin).String()))
				return nil, err
			}
			logger.Info("connection open", log.String("latency", time.Since(begin).String()))
			atomic.AddInt64(tran.connCounter, 1)
			callback(atomic.LoadInt64(tran.connCounter))
			instrumentedConn := &connWithCounter{ //nolint:exhaustruct
				Conn:        conn,
				connCounter: &counter,
				callback:    callback,
				logger:      logger,
			}
			return instrumentedConn, nil
		}
	}
	tran.Transport.DialContext = dialWithCounter(prevDialer)
	tran.Transport.DialTLSContext = dialWithCounter(prevTLSDialer)
	return tran
}

func (tran *Transport) getPreviousDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	if tran.DialContext != nil {
		return tran.DialContext
	}
	if tran.Dial != nil {
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return tran.Dial(network, addr) //nolint:wrapcheck
		}
	}
	var defaultDialer net.Dialer
	return defaultDialer.DialContext
}

func (tran *Transport) getPreviousTLSDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	if tran.DialTLSContext != nil {
		return tran.DialTLSContext
	}
	if tran.DialTLS != nil {
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return tran.DialTLS(network, addr) //nolint:wrapcheck
		}
	}
	return nil // DialContext will be used (see go/src/net/http/transport.go:144)
}

type connWithCounter struct {
	net.Conn
	closeOnce   sync.Once
	connCounter *int64
	callback    func(int64)
	logger      log.MetaLogger
}

func (conn *connWithCounter) Close() error {
	err := conn.Conn.Close()
	conn.closeOnce.Do(func() {
		atomic.AddInt64(conn.connCounter, -1)
		conn.callback(atomic.LoadInt64(conn.connCounter))
		conn.logger.Info("connection closed")
	})
	return err //nolint:wrapcheck
}
