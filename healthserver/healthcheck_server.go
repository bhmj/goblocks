package appstatus

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

type HealthcheckServer struct {
	server    *http.Server
	appStatus AppStatus
	logger    log.MetaLogger
	port      int
}

func NewHealthcheckServer(logger log.MetaLogger, port int, appStatus AppStatus) *HealthcheckServer {
	health := &HealthcheckServer{appStatus: appStatus, logger: logger, port: port}
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

func (s *HealthcheckServer) Run() error {
	s.logger.Info("starting healthcheck server",
		log.Bool("tls", false),
		log.Int("port", s.port),
	)

	if err := s.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			s.logger.Info("healthcheck server closed")
			return nil
		}
		return fmt.Errorf("failed to start healthcheck server: %w", err)
	}
	return nil
}

func (s *HealthcheckServer) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}

func (s *HealthcheckServer) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)

	if s.appStatus.IsReady() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *HealthcheckServer) AliveHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)

	if s.appStatus.IsAlive() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
