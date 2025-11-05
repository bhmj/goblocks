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
	RegisterService(name string, cfg any, factory ServiceFactory) error // service name must match the unquoted yaml key format (e.g. [a-zA-Z_]+)
	Run(config any)
}

// HandlerDefinition contains method definition to use by HTTP server
type HandlerDefinition struct {
	Endpoint string // used as "method" label for the `servicename_request_latency{method="endpoint"}` metric
	Method   string // GET, POST, etc.
	Path     string // URL path (Gorilla URL vars allowed)
	Func     httpserver.HandlerWithResult
}

// Service is an interface that application services should implement
type Service interface {
	GetHandlers() []HandlerDefinition
	Run(ctx context.Context) error
}

// AppInfo contains general app information and settings
type Options struct {
	Logger          log.MetaLogger
	MetricsRegistry *metrics.Registry
	ServiceReporter appstatus.ServiceStatusReporter
	Production      bool
	ConfigPath      string
}

// ServiceFactory is a function that creates a service instance
type ServiceFactory func(
	cfg any,
	options Options,
) (Service, error)
