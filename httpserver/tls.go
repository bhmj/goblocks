package httpserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
)

const (
	minTLSVersion = tls.VersionTLS12
	maxTLSVersion = tls.VersionTLS13
)

var (
	errFailedToReadCACertificate  = errors.New("failed to read CA certificate")
	errFailedToParseCACertificate = errors.New("failed to parse CA certificate")
	errDecodedPEMIsBlank          = errors.New("decoded PEM is blank")
	errNilCertificate             = errors.New("nil certificate")
)

type ListenerConfig struct {
	Address          string
	TLSEnabled       bool
	TLSCertFile      string
	TLSKeyFile       string
	TLSUseClientCert bool
	TLSClientCAFile  string
}

// InitListener preloads certificates and returns a configured net.Listener
func InitListener(cfg Config) (net.Listener, error) {
	var certFile string
	var err error

	if cfg.UseTLS {
		certFile, err = cfg.CertFile()
		if err != nil {
			return nil, fmt.Errorf("preload certificates: %w", err)
		}
	}

	lnConfig := &ListenerConfig{
		Address:          fmt.Sprintf(":%d", cfg.Port),
		TLSEnabled:       cfg.UseTLS,
		TLSCertFile:      certFile,
		TLSKeyFile:       cfg.TLSKey,
		TLSUseClientCert: cfg.TLSUseClientCert,
		TLSClientCAFile:  cfg.TLSClientCA,
	}

	return newListener(lnConfig)
}

// newListener creates and configures net.Listener
func newListener(lnConfig *ListenerConfig) (net.Listener, error) {
	ln, err := net.Listen("tcp", lnConfig.Address) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("listener: %w", err)
	}

	if !lnConfig.TLSEnabled {
		return ln, nil
	}

	tlsConfig, err := prepareTLSConfig(lnConfig)
	if err != nil {
		return nil, fmt.Errorf("prepare TLS config: %w", err)
	}

	return tls.NewListener(ln, tlsConfig), nil
}

func prepareTLSConfig(l *ListenerConfig) (*tls.Config, error) {
	cg := NewCertificateGetter(l.TLSCertFile, l.TLSKeyFile)
	if err := cg.Load(); err != nil {
		return nil, fmt.Errorf("load certificate: %w", err)
	}

	tlsConf := &tls.Config{
		MinVersion:     minTLSVersion,
		MaxVersion:     maxTLSVersion,
		GetCertificate: cg.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
		ClientAuth:     tls.RequestClientCert,
	}

	if l.TLSUseClientCert {
		err := setupClientCA(tlsConf, l)
		if err != nil {
			return nil, err
		}
	}

	return tlsConf, nil
}

func setupClientCA(tlsConf *tls.Config, l *ListenerConfig) error {
	tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
	if l.TLSClientCAFile != "" {
		caPool := x509.NewCertPool()
		data, err := os.ReadFile(l.TLSClientCAFile)
		if err != nil {
			return errFailedToReadCACertificate
		}

		if !caPool.AppendCertsFromPEM(data) {
			return errFailedToParseCACertificate
		}
		tlsConf.ClientCAs = caPool
	}
	return nil
}

// CertificateGetter allows to load certificates. The GetCertificate method
// satisfies the tls.GetCertificate function signature.
type CertificateGetter struct {
	cert     *tls.Certificate
	certFile string
	keyFile  string
}

func NewCertificateGetter(certFile, keyFile string) *CertificateGetter {
	return &CertificateGetter{
		certFile: certFile,
		keyFile:  keyFile,
	}
}

func (cg *CertificateGetter) Load() error {
	certPEMBlock, err := os.ReadFile(cg.certFile)
	if err != nil {
		return err //nolint:wrapcheck
	}
	keyPEMBlock, err := os.ReadFile(cg.keyFile)
	if err != nil {
		return err //nolint:wrapcheck
	}

	// Check for encrypted pem block
	keyBlock, _ := pem.Decode(keyPEMBlock)
	if keyBlock == nil {
		return errDecodedPEMIsBlank
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return err //nolint:wrapcheck
	}

	cg.cert = &cert

	return nil
}

func (cg *CertificateGetter) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if cg.cert == nil {
		return nil, errNilCertificate
	}

	return cg.cert, nil
}
