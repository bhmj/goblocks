package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/bhmj/goblocks/apiauth"
	"github.com/bhmj/goblocks/apiauth/token"
	"github.com/bhmj/goblocks/log"
	"github.com/bhmj/goblocks/metrics"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"golang.org/x/time/rate"
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

const rateLimitBurstRatio = float64(1.2) // allow this % bursts of incoming requests

// Server implements basic Kube-dispatched HTTP server
type Server interface {
	Run(ctx context.Context) error
	HandleFunc(service, endpoint, method, path string, handler HandlerWithResult)
}

type httpserver struct {
	name    string
	cfg     Config
	router  Router
	server  *http.Server
	logger  log.MetaLogger
	metrics *serviceMetrics

	listener net.Listener
}

// NewServer returns an HTTP server
func NewServer(
	cfg Config,
	cfgMetrics metrics.Config,
	router Router,
	logger log.MetaLogger,
	metricsRegistry *metrics.Registry,
	sentryHandler *sentryhttp.Handler,
) (Server, error) {
	metrics := newMetrics(metricsRegistry.Get(), cfgMetrics)

	connWatcher := NewConnectionWatcher(metricsRegistry.Get(), logger)
	rateLimiter := rate.NewLimiter(cfg.RateLimit, int(float64(cfg.RateLimit)*rateLimitBurstRatio))
	var authProvider apiauth.Auth
	if cfg.Token != "" {
		authProvider = token.New(cfg.Token)
	}

	// middlewares sequence (in order of execution during request handling):
	//
	// -> http server
	//
	// connection limiting
	// rate limiting
	// authentication
	// sentry handler
	// panic logging (logs panic and repanics for sentry)
	//
	// -> ROUTER (determine the necessity of further processing)
	//
	// instrumentation = request ID + logging + metrics + errorer
	//
	// -> SERVICE HANDLER

	safetyWrappers := func(router Router) http.Handler {
		return connLimiterMiddleware(
			rateLimiterMiddleware(
				authMiddleware(
					sentryHandler.HandleFunc(
						panicLoggerMiddleware(router, logger),
					),
					authProvider,
				),
				rateLimiter,
			),
			connWatcher,
			cfg.OpenConnLimit,
		)
	}

	srv := &httpserver{
		name:    "http",
		logger:  logger,
		metrics: metrics,
		cfg:     cfg,
		router:  router,
		server: &http.Server{
			ReadTimeout: cfg.ReadTimeout,
			ConnState:   connWatcher.OnStateChange,
			Handler:     safetyWrappers(router),
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
func (s *httpserver) Run(ctx context.Context) error {
	s.server.Addr = fmt.Sprintf(":%d", s.cfg.Port)
	s.logger.Info("starting server",
		log.String("name", s.name),
		log.Bool("tls", s.cfg.UseTLS),
		log.Bool("client_auth", s.cfg.TLSUseClientCert),
		log.Int("port", s.cfg.Port),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(s.listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			s.logger.Info("server closed", log.String("name", s.name))
			return nil
		}
		return fmt.Errorf("failed to start server: %w", err)
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		err := s.server.Shutdown(ctx) //nolint:contextcheck
		if err != nil {
			s.logger.Error("failed to shutdown server", log.String("name", s.name), log.Error(err))
		}
		return nil
	}
}

func (s *httpserver) HandleFunc(service, endpoint, method, path string, handler HandlerWithResult) {
	s.router.HandleFunc(
		method,
		"/"+strings.TrimPrefix(path, "/"),
		instrumentationMiddleware(handler, s.logger, s.metrics, service, endpoint),
	)
}
