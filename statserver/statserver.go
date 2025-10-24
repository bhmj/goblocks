package statserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bhmj/goblocks/log"
)

type AppStatus interface {
	IsReady() bool
	IsAlive() bool
}

type statServer struct {
	server    *http.Server
	appStatus AppStatus
	logger    log.MetaLogger
	port      int
}

type Server interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

func New(port int, logger log.MetaLogger, appStatus AppStatus, promHandler http.Handler) Server {
	result := &statServer{appStatus: appStatus, logger: logger, port: port}
	router := http.NewServeMux()
	router.HandleFunc("GET /ready", result.ReadyHandler)
	router.HandleFunc("GET /alive", result.AliveHandler)
	router.Handle("GET /metrics", promHandler)
	result.server = &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		ReadHeaderTimeout: time.Second,
		Handler:           router,
	}

	return result
}

func (s *statServer) Run(ctx context.Context) error {
	s.logger.Info("starting healthcheck server",
		log.Bool("tls", false),
		log.Int("port", s.port),
	)

	errCh := make(chan error)

	go func() {
		err := s.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			s.logger.Info("healthcheck server closed")
			errCh <- nil
			return
		}
		errCh <- fmt.Errorf("failed to start healthcheck server: %w", err)
	}()

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return s.server.Shutdown(ctx) //nolint:contextcheck
	case err := <-errCh:
		return err
	}
}

func (s *statServer) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}

func (s *statServer) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)

	if s.appStatus.IsReady() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *statServer) AliveHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)

	if s.appStatus.IsAlive() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
