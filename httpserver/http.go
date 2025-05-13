package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"golang.org/x/time/rate"

	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/bhmj/goblocks/auth"
	"github.com/bhmj/goblocks/auth/token"
	"github.com/bhmj/goblocks/log"
)

// Router implements a basic router interface. Currently in this repo
// you can find a gorilla/mux router wrapper and a standard ServeMux router wrapper.
// You can create a wrapper for your favourite router/multiplexer and pass it as
// Router to a NewServer() func.
type Router interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	// Handle(pattern string, handler http.Handler)
	HandleFunc(method, pattern string, handler func(http.ResponseWriter, *http.Request))
}

const (
	shutdownTimeout     = 1 * time.Second // how much time to wait for running queries until force server shutdown
	rateLimitBurstRatio = float64(1.2)    // allow this % bursts of incoming requests
)

// Server implements basic Kube-dispatched HTTP server
type Server interface {
	Run() error
	Shutdown(ctx context.Context) error
	HandleFunc(method, pattern string, handler http.HandlerFunc)
}

type httpserver struct {
	name   string
	cfg    Config
	router Router
	server *http.Server
	logger log.MetaLogger

	listener net.Listener

	sentryHandler *sentryhttp.Handler
	connWatcher   *ConnectionWatcher
	rateLimiter   *rate.Limiter
	authProvider  auth.Auth
}

// NewServer returns an HTTP server
func NewServer(
	cfg Config,
	router Router,
	logger log.MetaLogger,
	metricsRegistry prometheus.Registerer,
	sentryHandler *sentryhttp.Handler,
) (Server, error) {
	connWatcher := NewConnectionWatcher(metricsRegistry, logger)
	limiter := rate.NewLimiter(cfg.RateLimit, int(float64(cfg.RateLimit)*rateLimitBurstRatio))
	var authProvider auth.Auth
	if cfg.Token != "" {
		authProvider = token.New(cfg.Token)
	}
	srv := &httpserver{
		name:          "http",
		logger:        logger,
		cfg:           cfg,
		router:        router,
		sentryHandler: sentryHandler,
		connWatcher:   connWatcher,
		rateLimiter:   limiter,
		authProvider:  authProvider,
		server: &http.Server{
			ReadTimeout: cfg.ReadTimeout,
			Handler:     router,
			ConnState:   connWatcher.OnStateChange,
		},
	}

	var err error
	srv.listener, err = InitListener(cfg)
	if err != nil {
		logger.Error(err.Error())
		return nil, fmt.Errorf("init listener: %w", err)
	}

	return srv, nil
}

// Run the server
func (s *httpserver) Run() error {
	// set server port
	s.server.Addr = fmt.Sprintf(":%d", s.cfg.Port)
	s.logger.Info("starting server",
		log.String("name", s.name),
		log.Bool("tls", s.cfg.UseSSL),
		log.Bool("client_auth", s.cfg.SSLUseClientCert),
		log.Int("port", s.cfg.Port),
	)

	// server startup
	if err := s.server.Serve(s.listener); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			s.logger.Info(s.name + " server closed")
			return nil
		}
		return fmt.Errorf("failed to start %s server: %w", s.name, err)
	}

	return nil
}

func (s *httpserver) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}

// HandleFunc adds the following middleware:
//   - Sentry panic wrapper and recoverer
//   - incoming connection limiter
//   - request rate limiter (throttler)
//   - authentication
func (s *httpserver) HandleFunc(method, path string, handler http.HandlerFunc) {
	wrapped := s.sentryHandler.HandleFunc(
		ConnLimiterMiddleware(
			RateLimiterMiddleware(
				AuthenticationMiddleware(handler, s.authProvider),
				s.rateLimiter,
			),
			s.connWatcher,
			s.cfg.OpenConnLimit,
		),
	)
	s.router.HandleFunc(method, path, wrapped)
}
