package statsig

import (
	"strconv"
	"sync"
)

type SDKConfigs struct {
	flags   map[string]bool
	configs map[string]interface{}
	mu      sync.RWMutex
}

func (s *SDKConfigs) SetFlags(newFlags map[string]bool) {
	s.mu.Lock()
	s.flags = newFlags
	s.mu.Unlock()
}

func (s *SDKConfigs) SetConfigs(newConfigs map[string]interface{}) {
	s.mu.Lock()
	s.configs = newConfigs
	s.mu.Unlock()
}

func (s *SDKConfigs) On(key string) (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.flags[key]
	return val, exists
}

func (s *SDKConfigs) GetConfigNumValue(config string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.configs[config]
	if !exists {
		return 0, false
	}

	return getNumericValue(value)
}

func (s *SDKConfigs) GetConfigIntValue(config string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.configs[config]
	if !exists {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func (s *SDKConfigs) GetConfigStrValue(config string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.configs[config]
	if !exists {
		return "", false
	}

	switch v := value.(type) {
	case string:
		return v, true
	case int:
		return strconv.Itoa(v), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	default:
		return "", false
	}
}
