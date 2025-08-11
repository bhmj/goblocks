package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/log"
)

type PrometheusServer struct {
	httpServer *http.Server
	logger     log.MetaLogger
	config     httpserver.Config
	port       int
}

func New(logger log.MetaLogger, handler http.Handler, config httpserver.Config) *PrometheusServer {
	router := http.NewServeMux()
	router.Handle("GET /metrics", handler)
	server := &http.Server{
		Addr:        fmt.Sprintf(":%d", config.Port),
		ReadTimeout: config.ReadTimeout,
		Handler:     router,
	}

	return &PrometheusServer{
		httpServer: server,
		logger:     logger,
		config:     config,
		port:       config.Port,
	}
}

func (s *PrometheusServer) Run() error {
	s.logger.Info("starting prometheus server",
		log.Bool("tls", false),
		log.Int("port", s.port),
	)

	listener, err := httpserver.InitListener(s.config)
	if err != nil {
		return fmt.Errorf("failed to init listener: %w", err)
	}
	if err := s.httpServer.Serve(listener); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			s.logger.Info("prometheus server closed")
			return nil
		}
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func (s *PrometheusServer) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}
