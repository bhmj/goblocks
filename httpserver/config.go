package httpserver

import (
	"fmt"
	"os"
	"time"

	"github.com/bhmj/goblocks/metrics"
	"golang.org/x/time/rate"
)

type UnknownCNBehavior string

const (
	UCNAllow UnknownCNBehavior = "allow"
	UCNWarn  UnknownCNBehavior = "warn"
	UCNBlock UnknownCNBehavior = "block"
)

// Config defines server parameters
type Config struct {
	Port             int            `yaml:"port" description:"Port number API listens on" default:"8080"`
	StatsPort        int            `yaml:"stats_port" description:"Port number stats server listens on" default:"8081"`
	UseTLS           bool           `yaml:"use_tls" description:"Use TLS for API calls"`
	TLSCert          string         `yaml:"tls_cert" description:"API TLS cert location"`
	TLSKey           string         `yaml:"tls_key" description:"API TLS key location"`
	TLSCA            string         `yaml:"tls_ca" description:"Optional CA certificate"` //nolint:tagliatelle
	TLSUseClientCert bool           `yaml:"tls_use_client_cert" description:"Require and verify client certificate"`
	TLSClientCA      string         `yaml:"tls_client_ca" description:"Certificate Authority file for checking the authenticity of client"`
	CORS             bool           `yaml:"cors" description:"Allow CORS"`
	Token            string         `yaml:"token" description:"Secret auth token"`
	RateLimit        rate.Limit     `yaml:"rate_limit" description:"Rate limit (RPS)" default:"10000"`
	OpenConnLimit    int            `yaml:"open_conn_limit" description:"Open incoming connection limit" default:"1000"`
	ReadTimeout      time.Duration  `yaml:"read_timeout" description:"Server read timeout (closes idle keep-alive connection)" default:"5m"`
	ShutdownTimeout  time.Duration  `yaml:"shutdown_timeout" description:"Server shutdown timeout" default:"2s"`
	Metrics          metrics.Config `yaml:"metrics" description:"Server metrics configuration"`
}

// CertFile returns filename of TLS certificate containing
// the server certificate alone or (for CA-based key) both
// CA certificate and server certificate.
func (t *Config) CertFile() (string, error) {
	ca, err := os.ReadFile(t.TLSCA)
	if os.IsNotExist(err) {
		return t.TLSCert, nil
	}
	if err != nil {
		return "", fmt.Errorf("read CA certificate: %w", err)
	}
	cert, err := os.ReadFile(t.TLSCert)
	if err != nil {
		return "", fmt.Errorf("read server certificate: %w", err)
	}
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", fmt.Errorf("create temporary file for CA and server certificate: %w", err)
	}
	defer f.Close()
	_, err = f.Write(cert)
	if err != nil {
		return "", fmt.Errorf("append server certificate: %w", err)
	}
	_, err = f.WriteString("\n")
	if err != nil {
		return "", fmt.Errorf("append certificate separator: %w", err)
	}
	_, err = f.Write(ca)
	if err != nil {
		return "", fmt.Errorf("append CA certificate: %w", err)
	}
	return f.Name(), f.Close()
}
