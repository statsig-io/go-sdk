package statsig

import (
	"errors"
	"sync"
	"time"
)

const defaultTTLSetMaxSize = 100000
const minTTLSetMaxSize = 10000

type TTLSet struct {
	store                     map[string]struct{}
	maxSize                   int
	mu                        sync.RWMutex
	resetInterval             time.Duration
	shutdown                  bool
	isBackgroundThreadRunning bool
}

func NewTTLSet() *TTLSet {
	return NewTTLSetWithMaxSize(defaultTTLSetMaxSize)
}

func NewTTLSetWithMaxSize(maxSize int) *TTLSet {
	if maxSize <= 0 {
		maxSize = defaultTTLSetMaxSize
	}
	maxSize = enforceMinTTLSetMaxSize(maxSize)
	set := &TTLSet{
		store:         make(map[string]struct{}, maxSize),
		maxSize:       maxSize,
		resetInterval: time.Minute,
	}

	return set
}

func enforceMinTTLSetMaxSize(maxSize int) int {
	if maxSize < minTTLSetMaxSize {
		return minTTLSetMaxSize
	}
	return maxSize
}

func (s *TTLSet) Add(key string) {
	s.mu.Lock()
	if _, exists := s.store[key]; exists {
		s.mu.Unlock()
		return
	}
	if s.maxSize > 0 && len(s.store) >= s.maxSize {
		s.store = make(map[string]struct{}, s.maxSize)
	}
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
			s.store = make(map[string]struct{}, s.maxSize)
			s.mu.Unlock()
		}
	}()
}
