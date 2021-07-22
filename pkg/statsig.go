// A global singleton for interfacing with Statsig Feature Gates, Dynamic Configs, Experiments, and Event Logging
package statsig

import (
	"fmt"
	"statsig/pkg/types"
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
// Only returns an error if Initialize() has not been called
func CheckGate(user types.StatsigUser, gate string) (bool, error) {
	if instance == nil {
		return false, fmt.Errorf("must Initialize() statsig before calling CheckGate")
	}
	return instance.CheckGate(user, gate), nil
}

// Gets the DynamicConfig value for the given user
// Only returns an error if Initialize() has not been called
func GetConfig(user types.StatsigUser, config string) (types.DynamicConfig, error) {
	if instance == nil {
		return types.DynamicConfig{Name: config}, fmt.Errorf("must Initialize() statsig before calling GetConfig")
	}
	return instance.GetConfig(user, config), nil
}

// Gets the DynamicConfig value of an Experiment for the given user
// Only returns an error if Initialize() has not been called
func GetExperiment(user types.StatsigUser, experiment string) (types.DynamicConfig, error) {
	if instance == nil {
		return types.DynamicConfig{Name: experiment}, fmt.Errorf("must Initialize() statsig before calling GetExperiment")
	}
	return instance.GetExperiment(user, experiment), nil
}

// Logs an event to the Statsig console
func LogEvent(event types.StatsigEvent) error {
	if instance == nil {
		return fmt.Errorf("must Initialize() statsig before calling LogEvent")
	}
	instance.LogEvent(event)
	return nil
}

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func Shutdown() {
	instance.Shutdown()
}
