package statsig

import (
	"errors"
	"sync"
	"time"
)

type TTLSet struct {
	store         map[string]struct{}
	mu            sync.RWMutex
	resetInterval time.Duration
	shutdown      bool
}

func NewTTLSet() *TTLSet {
	set := &TTLSet{
		store:         make(map[string]struct{}),
		resetInterval: time.Minute,
	}

	go set.startResetThread()
	return set
}

func (s *TTLSet) Add(key string) {
	s.mu.Lock()
	s.store[key] = struct{}{}
	s.mu.Unlock()
}

func (s *TTLSet) Contains(key string) bool {
	s.mu.RLock()
	_, exists := s.store[key]
	s.mu.RUnlock()
	return exists
}

func (s *TTLSet) Reset() {
	s.mu.Lock()
	s.store = make(map[string]struct{})
	s.mu.Unlock()
}

func (s *TTLSet) Shutdown() {
	s.mu.Lock()
	s.shutdown = true
	s.mu.Unlock()
}

func (s *TTLSet) startResetThread() {
	for {
		time.Sleep(s.resetInterval)
		stop := func() bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.shutdown
		}()
		if stop {
			break
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					err := errors.New("panic in TTLSet reset thread")
					Logger().LogError(err)
				}
			}()
			s.Reset()
		}()
	}
}
