// Package statsig implements feature gating and a/b testing
package statsig

import (
	"fmt"
	"statsig/types"
	"sync"
)

var instance *Client
var once sync.Once

// Initializes the global Statsig instance with the given sdkKey
func Initialize(sdkKey string) {
	once.Do(func() {
		instance = NewClient(sdkKey)
	})
}

// Initializes the global Statsig instance with the given sdkKey and options
func InitializeWithOptions(sdkKey string, options *types.StatsigOptions) {
	once.Do(func() {
		instance = NewWithOptions(sdkKey, options)
	})
}

// Checks the value of a Feature Gate for the given user
func CheckGate(user types.StatsigUser, gate string) bool {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling CheckGate"))
	}
	return instance.CheckGate(user, gate)
}

// Gets the DynamicConfig value for the given user
func GetConfig(user types.StatsigUser, config string) types.DynamicConfig {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling GetConfig"))
	}
	return instance.GetConfig(user, config)
}

// Gets the DynamicConfig value of an Experiment for the given user
func GetExperiment(user types.StatsigUser, experiment string) types.DynamicConfig {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperiment"))
	}
	return instance.GetExperiment(user, experiment)
}

// Logs an event to the Statsig console
func LogEvent(event types.StatsigEvent) {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling LogEvent"))
	}
	instance.LogEvent(event)
}

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func Shutdown() {
	if instance == nil {
		return
	}
	instance.Shutdown()
}
