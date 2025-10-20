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
	alive bool
	ready map[string]bool
}

type serviceReporter struct {
	statusReporter *statusReporter
	serviceName    string
}

// ---------------------------------------------------------------

func New() StatusReporter {
	return &statusReporter{
		ready: make(map[string]bool),
		alive: true,
	}
}

func (s *statusReporter) GetServiceReporter(serviceName string) (ServiceStatusReporter, error) {
	s.Lock()
	defer s.Unlock()
	if _, found := s.ready[serviceName]; found {
		return nil, errAlreadyRegistered
	}
	s.ready[serviceName] = false
	return &serviceReporter{
		statusReporter: s,
		serviceName:    serviceName,
	}, nil
}

func (s *statusReporter) Ready(name string) {
	s.Lock()
	defer s.Unlock()
	s.ready[name] = true
}

func (s *statusReporter) NotReady(name string) {
	s.Lock()
	defer s.Unlock()
	s.ready[name] = false
}

func (s *statusReporter) Dead() {
	s.Lock()
	defer s.Unlock()
	s.alive = false
}

func (s *statusReporter) IsReady() bool {
	s.RLock()
	defer s.RUnlock()
	for _, ready := range s.ready {
		if !ready {
			return false
		}
	}
	return true
}

func (s *statusReporter) IsAlive() bool {
	s.RLock()
	defer s.RUnlock()
	return s.alive
}

func (s *serviceReporter) Ready() {
	s.statusReporter.Lock()
	defer s.statusReporter.Unlock()
	s.statusReporter.ready[s.serviceName] = true
}

func (s *serviceReporter) NotReady() {
	s.statusReporter.Lock()
	defer s.statusReporter.Unlock()
	s.statusReporter.ready[s.serviceName] = false
}

func (s *serviceReporter) Dead() {
	s.statusReporter.Lock()
	defer s.statusReporter.Unlock()
	s.statusReporter.alive = false
}
