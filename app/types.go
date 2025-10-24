package app

import (
	"context"

	"github.com/bhmj/goblocks/appstatus"
	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/log"
	"github.com/bhmj/goblocks/metrics"
)

// Application is the main application interface
type Application interface {
	RegisterService(name string, cfg any, factory ServiceFactory) error
	Run(config any)
}

// HandlerDefinition contains method definition to use by HTTP server
type HandlerDefinition struct {
	EndpointName string // Prometheus metric name for the endpoint (valid regex is [a-zA-Z_:][a-zA-Z0-9_:]* but good practice is to avoid colons!)
	Method       string // GET, POST, etc.
	Path         string // URL path (Gorilla URL vars allowed)
	Func         httpserver.HandlerWithResult
}

// Service is an interface that application services should implement
type Service interface {
	GetHandlers() []HandlerDefinition
	Run(ctx context.Context) error
}

// ServiceFactry is a function that creates a service instance
type ServiceFactory func(
	cfg any,
	logger log.MetaLogger,
	metricsRegistry *metrics.Registry,
	statusReporter appstatus.ServiceStatusReporter,
) (Service, error)
