package statsig

import (
	"sync"
	"time"
)

type errorContext struct {
	evalContext  *evalContext
	Caller       string `json:"tag,omitempty"`
	BypassDedupe bool
	LogToOutput  bool
	EventCount   int
}

type evalContext struct {
	Caller                     string `json:"tag,omitempty"`
	ConfigName                 string `json:"configName,omitempty"`
	ClientKey                  string `json:"clientKey,omitempty"`
	Hash                       string `json:"hash,omitempty"`
	TargetAppID                string
	IncludeLocalOverrides      bool
	IsManualExposure           bool
	IsExperiment               bool
	DisableLogExposures        bool
	PersistedValues            UserPersistedValues
	IncludeConfigType          bool
	ConfigTypesToInclude       []ConfigType
	EvalSamplingRate           *int
	EvalHasSeenAnalyticalGates bool
}

type initContext struct {
	Start   time.Time
	Success bool
	Error   error
	Source  EvaluationSource
	mu      sync.RWMutex
}

func newInitContext() *initContext {
	return &initContext{Start: time.Now(), Success: false, Source: SourceUninitialized}
}

func (c *initContext) setSuccess(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Success = success
}

func (c *initContext) setError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Error = err
}

func (c *initContext) setSource(source EvaluationSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Source = source
}

func (c *initContext) copy() *initContext {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &initContext{
		Start:   c.Start,
		Success: c.Success,
		Error:   c.Error,
		Source:  c.Source,
	}
}
