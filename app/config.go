package app

import (
	"time"

	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/sentry"
)

type Config struct {
	HTTP          httpserver.Config `yaml:"http" group:"HTTP endpoint configuration" default:"8080"`
	Sentry        sentry.Config     `yaml:"sentry" group:"Sentry configuration"`
	ShutdownDelay time.Duration     `yaml:"shutdown_delay" description:"" default:"2s"`
	LogLevel      string            `yaml:"log_level" description:"Log level in production mode" default:"info" choices:"debug,info,warn,error,dpanic,panic,fatal"`
	Production    bool              `yaml:"production" description:"Production mode"`
}
