package appstatus

import (
	"errors"
	"sync"
)

var errAlreadyRegistered = errors.New("service already registered")

type StatusReporter interface {
	GetServiceReporter(serviceName string) (ServiceStatusReporter, error)
	IsReady() bool
	IsAlive() bool
}

type ServiceStatusReporter interface {
	Ready()    // service is up
	NotReady() // temporary outage; expecting recovery
	Dead()     // service is down
}

type statusReporter struct {
	sync.RWMutex
	appAlive     bool
	serviceReady map[string]bool
}

type serviceReporter struct {
	statusReporter *statusReporter
	serviceName    string
}

// ---------------------------------------------------------------

func New() StatusReporter {
	return &statusReporter{
		serviceReady: make(map[string]bool),
		appAlive:     true,
	}
}

func (s *statusReporter) GetServiceReporter(serviceName string) (ServiceStatusReporter, error) {
	s.Lock()
	defer s.Unlock()
	if _, found := s.serviceReady[serviceName]; found {
		return nil, errAlreadyRegistered
	}
	s.serviceReady[serviceName] = false
	return &serviceReporter{
		statusReporter: s,
		serviceName:    serviceName,
	}, nil
}

func (s *statusReporter) ready(name string) {
	s.Lock()
	defer s.Unlock()
	s.serviceReady[name] = true
}

func (s *statusReporter) notReady(name string) {
	s.Lock()
	defer s.Unlock()
	s.serviceReady[name] = false
}

func (s *statusReporter) dead() {
	s.Lock()
	defer s.Unlock()
	s.appAlive = false
}

func (s *statusReporter) IsReady() bool {
	s.RLock()
	defer s.RUnlock()
	for _, ready := range s.serviceReady {
		if !ready {
			return false
		}
	}
	return true
}

func (s *statusReporter) IsAlive() bool {
	s.RLock()
	defer s.RUnlock()
	return s.appAlive
}

func (s *serviceReporter) Ready() {
	s.statusReporter.ready(s.serviceName)
}

func (s *serviceReporter) NotReady() {
	s.statusReporter.notReady(s.serviceName)
}

func (s *serviceReporter) Dead() {
	s.statusReporter.dead()
}
