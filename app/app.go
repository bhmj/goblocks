package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	syslog "log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"syscall"
	"time"

	"github.com/bhmj/goblocks/appstatus"
	"github.com/bhmj/goblocks/conftool"
	"github.com/bhmj/goblocks/gorillarouter"
	"github.com/bhmj/goblocks/httpserver"
	"github.com/bhmj/goblocks/log"
	"github.com/bhmj/goblocks/metrics"
	"github.com/bhmj/goblocks/sentry"
	"github.com/bhmj/goblocks/statserver"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var (
	errInvalidServiceName = errors.New("service name must match the unquoted yaml key format (e.g. [a-zA-Z_]+)")
	errEmptyConfig        = errors.New("config is empty or invalid YAML document")
)

type application struct {
	services    map[string]Service
	serviceDefs map[string]registeredService
	logger      log.MetaLogger
	cfg         *Config
	cfgPath     string
	httpServer  httpserver.Server
	statServer  statserver.Server
}

type registeredService struct {
	Name    string
	Config  any
	Factory ServiceFactory
}

// New creates a new Application instance
func New(appName, appVersion string) Application {
	currentUser, err := user.Current()
	if err != nil {
		syslog.Fatal(err.Error())
	}
	syslog.Printf("Starting %s, version %s\n", appName, appVersion)
	syslog.Printf("username: %s, uid: %s, gid: %s", currentUser.Username, currentUser.Uid, currentUser.Gid)

	return &application{cfg: &Config{}}
}

// RegisterService registers a service with the application
func (a *application) RegisterService(name string, cfg any, factory ServiceFactory) error {
	reName := regexp.MustCompile("[a-zA-Z][a-zA-Z_]*")
	if !reName.MatchString(name) {
		return errInvalidServiceName
	}
	if a.serviceDefs == nil {
		a.serviceDefs = make(map[string]registeredService)
	}

	vcfg := cfg
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer {
		v := reflect.New(rv.Type())
		v.Elem().Set(rv)
		vcfg = v.Interface()
	}

	a.serviceDefs[name] = registeredService{
		Name:    name,
		Config:  vcfg,
		Factory: factory,
	}
	return nil
}

// Run starts the application. config is optional explicit config. If nil, config is read from file.
func (a *application) Run(config any) {
	// set GOMAXPROCS
	if _, err := maxprocs.Set(maxprocs.Logger(syslog.Printf)); err != nil {
		syslog.Fatal("failed to set GOMAXPROCS", "error", err)
	}

	// config
	if config != nil {
		a.readConfigStruct(config)
	} else {
		a.readConfigFile()
	}

	// app status
	appStatus := appstatus.New()
	appReporter, _ := appStatus.GetServiceReporter("app")

	// metrics registry
	metricsRegistry, err := metrics.NewRegistry(a.cfg.HTTP.Metrics)
	if err != nil {
		syslog.Fatal("metrics: %w", err)
	}

	// logger
	logger, err := log.New(a.cfg.LogLevel, false)
	if err != nil {
		syslog.Fatal(err.Error())
	}

	// sentry
	sentryService, err := sentry.NewService(a.cfg.Sentry)
	if err != nil {
		logger.Fatal("create sentry service", log.Error(err))
	}

	// router
	router := gorillarouter.New()

	// app + metrics http servers
	a.httpServer, err = httpserver.NewServer(a.cfg.HTTP, a.cfg.HTTP.Metrics, router, logger, metricsRegistry, sentryService.GetHandler())
	if err != nil {
		logger.Fatal("create app http server", log.Error(err))
	}
	a.statServer = statserver.New(a.cfg.HTTP.StatsPort, logger, appStatus, metricsRegistry.Handler())
	if err != nil {
		logger.Fatal("create stats http server", log.Error(err))
	}

	a.logger = logger

	// create services
	a.services = make(map[string]Service)
	for name, reg := range a.serviceDefs {
		serviceReporter, err := appStatus.GetServiceReporter(name)
		if err != nil {
			logger.Fatal("create service reporter", log.String("service", name), log.Error(err))
		}
		options := Options{
			Logger:          a.logger,
			MetricsRegistry: metricsRegistry,
			ServiceReporter: serviceReporter,
			ConfigPath:      a.cfgPath,
		}
		service, err := reg.Factory(reg.Config, options)
		if err != nil {
			logger.Fatal("create service", log.String("service", name), log.Error(err))
		}
		a.services[name] = service
	}

	a.runEverything(appReporter)

	a.logger.Sync() //nolint:errcheck
}

func (a *application) readConfigStruct(config any) {
	pwd, err := os.Getwd()
	if err != nil {
		syslog.Fatalf("get current dir: %s", err)
	}
	a.cfgPath = pwd

	err = a.applyConfigStruct(config)
	if err != nil {
		syslog.Fatalf("read config data: %s", err)
	}
}

func (a *application) readConfigFile() {
	pstr := flag.String("config-file", "", "")
	flag.Parse()
	if pstr == nil || *pstr == "" {
		syslog.Fatalf("Usage: %s --config-file=/path/to/config.yaml", os.Args[0])
	}
	a.cfgPath = filepath.Dir(*pstr)

	fullname, err := filepath.Abs(*pstr)
	if err != nil {
		syslog.Fatalf("filepath: %s", err)
	}
	raw, err := os.ReadFile(fullname)
	if err != nil {
		syslog.Fatalf("read config: %s", err)
	}

	data := conftool.ParseEnvVars(raw)
	err = a.readConfigData(data)
	if err != nil {
		syslog.Fatalf("read config data: %s", err)
	}
}

func (a *application) readConfigData(data []byte) error {
	var root yaml.Node

	cfg := make(map[string]any)

	for name, reg := range a.serviceDefs {
		cfg[name] = reg.Config
	}

	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	if len(root.Content) == 0 {
		return errEmptyConfig
	}

	mapping := root.Content[0] // top-level mapping
	rootNodes := make(map[string]*yaml.Node)
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		val := mapping.Content[i+1]
		rootNodes[key] = val
	}

	// decode app config
	if node, ok := rootNodes["app"]; ok {
		if err := node.Decode(a.cfg); err != nil {
			return fmt.Errorf("app config: %w", err)
		}
	}
	if err := conftool.DefaultsAndRequired(a.cfg); err != nil {
		return fmt.Errorf("app config: missing required value: %w", err)
	}

	// decode configs for all registered services
	for name, service := range a.serviceDefs {
		node, ok := rootNodes[name]
		if !ok {
			continue
		}
		if err := node.Decode(service.Config); err != nil {
			return fmt.Errorf("decode %s: %w", name, err)
		}
	}
	for name, service := range a.serviceDefs {
		if err := conftool.DefaultsAndRequired(service.Config); err != nil {
			return fmt.Errorf("%s config: missing required value: %w", name, err)
		}
	}

	return nil
}

// Run starts the application
func (a *application) runEverything(appReporter appstatus.ServiceStatusReporter) {
	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)

	// run api server + metrics server
	eg.Go(func() error {
		if err := a.statServer.Run(ctx); err != nil {
			return fmt.Errorf("prometheus server: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		if err := a.httpServer.Run(ctx); err != nil {
			return fmt.Errorf("api server: %w", err)
		}
		return nil
	})

	// run services
	for name, service := range a.services {
		a.addHandlers(name, service.GetHandlers())
		eg.Go(func() error {
			return service.Run(ctx)
		})
	}

	appReporter.Ready()

	// term handler
	eg.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		select {
		case <-ctx.Done():
			return nil
		case signal := <-ch:
			cancel()
			a.logger.Info("signal received", log.String("signal", signal.String()))
			a.logger.Info("shutting down", log.Duration("shutdown delay", a.cfg.ShutdownDelay), log.MainMessage())
			a.logger.Flush()
			return nil
		}
	})

	err := eg.Wait()
	appReporter.NotReady()
	time.Sleep(a.cfg.ShutdownDelay) // wait until k8s get to know it: see readinessProbe/periodSeconds in k8s config
	if err != nil {
		a.logger.Error("terminated with error", log.Error(err))
		return
	}

	a.logger.Info("terminated successfully")
}

func (a *application) addHandlers(service string, handlers []HandlerDefinition) {
	for _, h := range handlers {
		a.httpServer.HandleFunc(service, h.Endpoint, h.Method, h.Path, h.Func)
	}
}
