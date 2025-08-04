package limitmap

import "sync"

type LimitMap struct {
	sync.RWMutex
	lmap map[any]int
}

// New creates new key-based stats keeper
func New() *LimitMap {
	return &LimitMap{
		lmap: make(map[any]int),
	}
}

// Inc increases counter for the given key. Returns false if limit reached.
func (s *LimitMap) Inc(key any, limit int) bool {
	s.Lock()
	n := s.lmap[key]
	n++
	if n > limit {
		s.Unlock()
		return false
	}
	s.lmap[key] = n
	s.Unlock()
	return true
}

// Dec decreases counter for the given key.
func (s *LimitMap) Dec(key any) int {
	s.Lock()
	n := s.lmap[key]
	if n > 0 {
		n--
	}
	if n == 0 {
		delete(s.lmap, key)
	} else {
		s.lmap[key] = n
	}
	s.Unlock()
	return n
}

// Value returns value for the given key.
func (s *LimitMap) Value(key any) int {
	s.RLock()
	n := s.lmap[key]
	s.RUnlock()
	return n
}
