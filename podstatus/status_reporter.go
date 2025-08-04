package podstatus

import "sync"

type StatusReporter struct {
	stateMutex sync.RWMutex
	ready      bool
	alive      bool
}

func (s *StatusReporter) Ready() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.ready = true
}

func (s *StatusReporter) NotReady() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.ready = false
}

func (s *StatusReporter) IsReady() bool {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.ready
}

func (s *StatusReporter) Alive() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.alive = true
}

func (s *StatusReporter) Dead() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.alive = false
}

func (s *StatusReporter) IsAlive() bool {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.alive
}
