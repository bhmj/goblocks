package app

import (
	"time"

	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/sentry"
)

type Config struct {
	HTTP          httpserver.Config `yaml:"http" group:"HTTP endpoint configuration"`
	Sentry        sentry.Config     `yaml:"sentry" group:"Sentry configuration"`
	ShutdownDelay time.Duration     `yaml:"shutdownDelay" description:"Time to wait before shutting down"`
	LogLevel      string            `yaml:"logLevel" description:"Log level in production mode" default:"info" choices:"debug,info,warn,error,dpanic,panic,fatal"`
	Production    bool              `yaml:"production" description:"Production mode"`
}
