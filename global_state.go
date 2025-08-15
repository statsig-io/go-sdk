package statsig

import (
	"sync"

	"github.com/google/uuid"
)

// Using global state variables directly will lead to race conditions
// Instead, define an accessor below using the Mutex lock
type GlobalState struct {
	logger    *OutputLogger
	sessionID string
	mu        sync.RWMutex
}

var global GlobalState

func Logger() *OutputLogger {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.logger
}

func InitializeGlobalOutputLogger(options OutputLoggerOptions, observabilityClient IObservabilityClient) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.logger = &OutputLogger{
		options:             options,
		observabilityClient: observabilityClient,
	}
	global.logger.Initialize()
}

func SessionID() string {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.sessionID
}

func InitializeGlobalSessionID() {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.sessionID = uuid.NewString()
}
