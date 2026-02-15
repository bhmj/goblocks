package main

import (
	"context"
	"errors"
	"fmt"
	syslog "log"
	"math/big"
	"net/http"
	"strconv"

	"github.com/bhmj/goblocks/app"
	"github.com/bhmj/goblocks/appstatus"
	"github.com/bhmj/goblocks/httpreply"
	"github.com/bhmj/goblocks/log"
	"github.com/gorilla/mux"
)

const serviceName = "factorial"

// the service config data is located in the single yaml file under the "{service_name}" key
type serviceConfig struct {
	APIRoot   string `yaml:"apiRoot" default:"/api"`
	CountBits bool   `yaml:"countBits" default:"false"`
}

// main service structure
type serviceData struct {
	cfg            *serviceConfig
	logger         log.MetaLogger
	statusReporter appstatus.ServiceStatusReporter
}

// FactorialServiceFactory returns a ready-to-run service instance
func FactorialServiceFactory(
	cfg any,
	options app.Options,
) (app.Service, error) {
	config, ok := cfg.(*serviceConfig)
	if !ok {
		return nil, errors.New("invalid config type")
	}
	return &serviceData{
		cfg:            config,
		logger:         options.Logger,
		statusReporter: options.ServiceReporter, // this can and should be used to report service status
	}, nil
}

// GetHandlers returns HTTP handlers for the service
func (s *serviceData) GetHandlers() []app.HandlerDefinition {
	return []app.HandlerDefinition{
		{
			Endpoint: "factorial",
			Method:   "GET",
			Path:     s.cfg.APIRoot + "/factorial/{number:[0-9]+}",
			Func:     s.factorialHandler,
		},
	}
}

func (s *serviceData) GetSessionData(_ string) (any, error) {
	return nil, nil //nolint:nilnil
}

// sample service handler with business logic
func (s *serviceData) factorialHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	params := mux.Vars(r)
	number := params["number"]
	inumber, err := strconv.Atoi(number)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid number: %w", err)
	}

	result := s.factorial(inumber)

	if s.cfg.CountBits {
		bits := result.BitLen()
		response := map[string]int{"bits": bits}
		return httpreply.Object(w, response)
	}
	response := map[string]*big.Int{"factorial": result}
	return httpreply.Object(w, response)
}

func (s *serviceData) factorial(n int) *big.Int {
	bigOne := big.NewInt(1)
	bigNum := big.NewInt(int64(n))
	result := big.NewInt(1)
	for n > 1 {
		result.Mul(result, bigNum)
		bigNum.Sub(bigNum, bigOne)
		n--
	}
	return result
}

// Run formally starts the service
func (s *serviceData) Run(ctx context.Context) error {
	s.statusReporter.Ready() // IMPORTANT!
	<-ctx.Done()
	s.logger.Info("Service terminated")
	return nil
}

var appVersion = "0.1.0" //nolint:gochecknoglobals

func main() {
	app := app.New("factorial", appVersion)
	err := app.RegisterService(serviceName, &serviceConfig{}, FactorialServiceFactory)
	if err != nil {
		syslog.Fatalf("register service: %v", err)
	}
	app.Run(nil)
}
