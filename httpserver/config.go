package httpserver

import (
	"fmt"
	"os"
	"time"

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
	APIBase          string        `yaml:"api_base" description:"Public API base prefix" default:"api"`
	Port             int           `yaml:"port" description:"Port number API listens on" default:"8080"`
	HealthPort       int           `yaml:"health_port" description:"Port number health check listens on" default:"8082"`
	UseSSL           bool          `yaml:"use_ssl" description:"Use SSL for API calls"`
	SSLCert          string        `yaml:"ssl_cert" description:"API SSL cert location"`
	SSLKey           string        `yaml:"ssl_key" description:"API SSL key location"`
	SSLCA            string        `yaml:"ssl_ca" description:"Optional CA certificate"`
	SSLUseClientCert bool          `yaml:"ssl_use_client_cert" description:"Require and verify client certificate"`
	SSLClientCA      string        `yaml:"ssl_client_ca" description:"Certificate Authority file for checking the authenticity of client"` //nolint:lll
	CORS             bool          `yaml:"cors" description:"Allow CORS"`
	Token            string        `yaml:"token" description:"Secret auth token"`
	RateLimit        rate.Limit    `yaml:"rate_limit" description:"Rate limit (RPS)" default:"10000"`
	OpenConnLimit    int           `yaml:"open_conn_limit" description:"Open incoming connection limit" default:"1000"`
	ReadTimeout      time.Duration `yaml:"read_timeout" description:"Server read timeout (closes idle keep-alive connection)" default:"5m"` //nolint:lll
}

// CertFile returns filename of SSL certificate containing
// the server certificate alone or (for CA-based key) both
// CA certificate and server certificate.
func (t *Config) CertFile() (string, error) {
	ca, err := os.ReadFile(t.SSLCA)
	if os.IsNotExist(err) {
		return t.SSLCert, nil
	}
	if err != nil {
		return "", fmt.Errorf("read CA certificate: %w", err)
	}
	cert, err := os.ReadFile(t.SSLCert)
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
