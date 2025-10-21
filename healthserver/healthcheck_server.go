package healthserver

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

type Server struct {
	server    *http.Server
	appStatus AppStatus
	logger    log.MetaLogger
	port      int
}

func New(logger log.MetaLogger, port int, appStatus AppStatus) *Server {
	health := &Server{appStatus: appStatus, logger: logger, port: port}
	router := http.NewServeMux()
	router.HandleFunc("GET /ready", health.ReadyHandler)
	router.HandleFunc("GET /alive", health.AliveHandler)
	health.server = &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		ReadHeaderTimeout: time.Second,
		Handler:           router,
	}

	return health
}

func (s *Server) Run(ctx context.Context) error {
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
		return s.server.Shutdown(ctx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}

func (s *Server) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)

	if s.appStatus.IsReady() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) AliveHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)

	if s.appStatus.IsAlive() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
