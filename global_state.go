package statsig

import "sync"

// Using global state variables directly will lead to race conditions
// Instead, define an accessor below using the Mutex lock
type GlobalState struct {
	logger *OutputLogger
	mu     sync.RWMutex
}

var global GlobalState

func (g *GlobalState) Logger() *OutputLogger {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.logger
}

func InitializeGlobalOutputLogger(options OutputLoggerOptions) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.logger = &OutputLogger{
		options: options,
	}
}
