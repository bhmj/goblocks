package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/bhmj/goblocks/app"
	"github.com/bhmj/goblocks/appstatus"
	"github.com/bhmj/goblocks/httpreply"
	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/log"
	"github.com/bhmj/goblocks/metrics"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/assert"
)

// service implements business logic ---------------------------------------

const serviceName = "test_service"

// the service config data is located in the single yaml file under the "{service_name}" key
type serviceConfig struct {
	Setting string `yaml:"setting" default:"default setting"`
}

// this is optional but implemented here as an example
type serviceMetrics struct {
	apiLatency *prometheus.HistogramVec
}

// main service structure
type serviceData struct {
	cfg            *serviceConfig
	logger         log.MetaLogger
	metrics        *serviceMetrics
	statusReporter appstatus.ServiceStatusReporter
}

// sample metrics initialization method
func newServiceMetrics(registry prometheus.Registerer) *serviceMetrics {
	metrics := &serviceMetrics{}
	factory := promauto.With(registry)
	labelNames := []string{"method", "code"}

	metrics.apiLatency = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "test_latency",
		Help:    "Latency of test service API calls",
		Buckets: []float64{0.001, 0.002, 0.003, 0.004, 0.005},
	}, labelNames)

	return metrics
}

// FactoryForTestService returns a ready-to-run service instance
func FactoryForTestService(
	cfg any,
	logger log.MetaLogger,
	metricsRegistry *metrics.Registry,
	statusReporter appstatus.ServiceStatusReporter,
) (app.Service, error) {
	metrics := newServiceMetrics(metricsRegistry.Get())
	return &serviceData{
		cfg:            cfg.(*serviceConfig),
		logger:         logger,
		metrics:        metrics,
		statusReporter: statusReporter, // this can and should be used to report service status
	}, nil
}

// GetHandlers returns HTTP handlers for the service
func (s *serviceData) GetHandlers() []app.HandlerDefinition {
	return []app.HandlerDefinition{
		{
			EndpointName: "factorial",
			Method:       "GET",
			Path:         "/factorial/{number:[0-9a-z]+}", // the error in regex is deliberate for test purposes
			Func:         s.factorialHandler,
		},
		{
			EndpointName: "settings",
			Method:       "GET",
			Path:         "/settings",
			Func:         s.settingsHandler,
		},
	}
}

// sample service handler with business logic
func (s *serviceData) factorialHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	params := mux.Vars(r)
	number := params["number"]
	inumber, err := strconv.Atoi(number)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid number: %w", err)
	}
	result := 1
	for inumber > 1 {
		result *= inumber
		inumber--
	}
	response := map[string]int{"result": result}

	return httpreply.Object(w, response)
}

// sample service handler with business logic
func (s *serviceData) settingsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return httpreply.Object(w, s.cfg)
}

// Run formally starts the service
func (s *serviceData) Run(ctx context.Context) error {
	// here we can start background goroutines, database connections, etc.

	// Below is a very important line!
	// we MUST report service as ready otherwise k8s readiness check will fail.
	s.statusReporter.Ready()
	// It can be skipped if running without k8s, but it's a good practice to have it.

	<-ctx.Done()
	return nil
}

// tests -------------------------------------------------------------------

type TestConfig struct {
	App     app.Config    `yaml:"app"`
	Service serviceConfig `yaml:"test_service"`
}

func TestAppReady(t *testing.T) {
	a := assert.New(t)

	// prepare and run app
	cfg := CreateTestConfig()
	app := app.New("test app")
	serviceCfg := &serviceConfig{}
	err := app.RegisterService(serviceName, serviceCfg, FactoryForTestService)
	a.Nil(err)
	go func() { app.Run(cfg) }()

	// test readiness endpoint
	a.False(getReady(cfg.App.HTTP.StatsPort), "not ready yet")
	started := time.Now()
	for !getReady(cfg.App.HTTP.StatsPort) && time.Since(started) < time.Second {
		time.Sleep(1 * time.Millisecond)
	}
	t.Logf("ready in %v\n", time.Since(started))
	a.True(getReady(cfg.App.HTTP.StatsPort), "app must be ready")
}

func TestAppResponse(t *testing.T) {
	a := assert.New(t)

	// prepare and run app
	cfg := CreateTestConfig()
	app := app.New("test app")
	serviceCfg := &serviceConfig{}
	err := app.RegisterService(serviceName, serviceCfg, FactoryForTestService)
	a.NoError(err)
	go func() { app.Run(cfg) }()
	for !getReady(cfg.App.HTTP.StatsPort) {
		time.Sleep(10 * time.Millisecond)
	}

	// test factorial endpoint
	resp, err := getFactorial(cfg.App.HTTP.Port, 5)
	a.NoError(err)
	a.Equal(http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	a.NoError(err)
	var obj map[string]any
	err = json.Unmarshal(data, &obj)
	a.NoError(err)
	result := int(obj["result"].(float64))
	a.Equal(result, 5*4*3*2*1)

	// test metrics
	lines, err := getMetrics(cfg.App.HTTP.StatsPort)
	a.NoError(err)
	var found bool
	for _, line := range lines {
		if strings.HasPrefix(line, "testapp_request_latency_bucket") {
			found = true
			break
		}
	}
	a.True(found, "service metrics not found in /metrics output")
}

func TestAppShutdown(t *testing.T) {
	a := assert.New(t)

	// prepare and run app
	cfg := CreateTestConfig()
	app := app.New("test app")
	serviceCfg := &serviceConfig{}
	err := app.RegisterService(serviceName, serviceCfg, FactoryForTestService)
	a.NoError(err)
	go func() { app.Run(cfg) }()
	for !getReady(cfg.App.HTTP.StatsPort) {
		time.Sleep(10 * time.Millisecond)
	}

	// Send SIGTERM to *this* process
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("failed to find process: %v", err)
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	for getReady(cfg.App.HTTP.StatsPort) {
		time.Sleep(10 * time.Millisecond)
	}
	a.False(getReady(cfg.App.HTTP.StatsPort))
}

func CreateTestConfig() *TestConfig {
	ports := getFreeTCPPorts(2)
	return &TestConfig{
		App: app.Config{
			HTTP: httpserver.Config{
				APIBase:   "api",
				Port:      ports[0],
				StatsPort: ports[1],
				Metrics: metrics.Config{
					Namespace: "testapp",
				},
			},
		},
		Service: serviceConfig{
			Setting: "my setting",
		},
	}
}

func getReady(port int) bool {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/ready", port), nil)
	resp, err := httpClient.Do(req)
	return err == nil && resp.StatusCode == http.StatusOK
}

func getMetrics(port int) ([]string, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/metrics", port), nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func getFactorial(port, number int) (*http.Response, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/factorial/%d", port, number), nil)
	return httpClient.Do(req)
}

func getFreeTCPPorts(n int) []int {
	var ports []int
	for port := 10000; port < 65535; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			ports = append(ports, port)
			n--
			if n == 0 {
				return ports
			}
		}
	}
	panic(fmt.Sprintf("unable to get free ports (%d)", n))
}
