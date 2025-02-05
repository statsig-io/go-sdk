package statsig

import (
	"errors"
	"sync"
	"time"
)

type TTLSet struct {
	store                     map[string]struct{}
	mu                        sync.RWMutex
	resetInterval             time.Duration
	shutdown                  bool
	isBackgroundThreadRunning bool
}

func NewTTLSet() *TTLSet {
	set := &TTLSet{
		store:         make(map[string]struct{}),
		resetInterval: time.Minute,
	}

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

func (s *TTLSet) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdown = true
	s.isBackgroundThreadRunning = false
}

func (s *TTLSet) StartResetThread() {
	s.mu.Lock()
	if s.isBackgroundThreadRunning {
		s.mu.Unlock()
		return
	}
	s.isBackgroundThreadRunning = true
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := errors.New("panic in TTLSet reset thread")
				Logger().LogError(err)
			}
		}()
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

			s.mu.Lock()
			s.store = make(map[string]struct{})
			s.mu.Unlock()
		}
	}()
}
