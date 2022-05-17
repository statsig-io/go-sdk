// Package statsig implements feature gating and a/b testing
package statsig

import (
	"fmt"
	"net/http"
	"time"
)

const DefaultEndpoint = "https://statsigapi.net/v1"

var instance *Client

// Initializes the global Statsig instance with the given sdkKey
func Initialize(sdkKey string) {
	if instance != nil {
		fmt.Println("Statsig is already initialized.")
		return
	}

	instance = NewClient(sdkKey)
}

// Advanced options for configuring the Statsig SDK
type Options struct {
	API                string      `json:"api"`
	Environment        Environment `json:"environment"`
	LocalMode          bool        `json:"localMode"`
	ConfigSyncInterval time.Duration
	IDListSyncInterval time.Duration
}

// See https://docs.statsig.com/guides/usingEnvironments
type Environment struct {
	Tier   string            `json:"tier"`
	Params map[string]string `json:"params"`
}

// Initializes the global Statsig instance with the given sdkKey and options
func InitializeWithOptions(sdkKey string, options *Options) {
	if instance != nil {
		fmt.Println("Statsig is already initialized.")
		return
	}

	instance = NewClientWithOptions(sdkKey, options)
}

// Checks the value of a Feature Gate for the given user
func CheckGate(user User, gate string) bool {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling CheckGate"))
	}
	return instance.CheckGate(user, gate)
}

// Gets the DynamicConfig value for the given user
func GetConfig(user User, config string) DynamicConfig {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling GetConfig"))
	}
	return instance.GetConfig(user, config)
}

// Override the value of a Feature Gate for the given user
func OverrideGate(gate string, val bool) {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideGate"))
	}
	instance.OverrideGate(gate, val)
}

// Override the DynamicConfig value for the given user
func OverrideConfig(config string, val map[string]interface{}) {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideConfig"))
	}
	instance.OverrideConfig(config, val)
}

// Gets the DynamicConfig value of an Experiment for the given user
func GetExperiment(user User, experiment string) DynamicConfig {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperiment"))
	}
	return instance.GetExperiment(user, experiment)
}

// Gets the Layer object for the given user
func GetLayer(user User, layer string) Layer {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling GetLayer"))
	}
	return instance.GetLayer(user, layer)
}

// Logs an event to the Statsig console
func LogEvent(event Event) {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling LogEvent"))
	}
	instance.LogEvent(event)
}

// Logs a slice of events to Statsig server immediately
func LogImmediate(events []Event) (*http.Response, error) {
	if instance == nil {
		panic(fmt.Errorf("must Initialize() statsig before calling LogImmediate"))
	}
	return instance.LogImmediate(events)
}

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func Shutdown() {
	if instance == nil {
		return
	}
	instance.Shutdown()
}

// For test only so we can clear the shared instance. Not thread safe.
func shutDownAndClearInstance() {
	Shutdown()
	instance = nil
}
