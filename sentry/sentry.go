package sentry

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

type Service struct {
	Handler *sentryhttp.Handler
}

type Config struct {
	DSN string `long:"dsn" env:"DSN" description:"DSN string"`
}

func NewService(conf Config) (*Service, error) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              conf.DSN,
		AttachStacktrace: true,
	})
	if err != nil {
		return nil, fmt.Errorf("sentry init: %w", err)
	}

	return &Service{
		Handler: sentryhttp.New(sentryhttp.Options{}),
	}, nil
}

func (srv *Service) GetHandler() *sentryhttp.Handler {
	return srv.Handler
}

func (srv *Service) Flush(timeout time.Duration) bool {
	return sentry.Flush(timeout)
}
