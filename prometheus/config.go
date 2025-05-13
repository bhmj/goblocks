package prometheus

import (
	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/metrics"
)

type Config struct {
	Server  httpserver.Config `yaml:"server" description:"Prometheus server configuration"`
	Metrics metrics.Config    `yaml:"metrics" description:"Metrics settings"`
}
