// Package statsig implements feature gating and a/b testing
package statsig

import (
	"fmt"
	"net/http"
	"time"
)

var instance *Client

// Initialize initializes the global Statsig instance with the given sdkKey
func Initialize(sdkKey string) {
	InitializeGlobalOutputLogger(OutputLoggerOptions{})
	InitializeGlobalSessionID()
	if IsInitialized() {
		Logger().Log("Statsig is already initialized.", nil)
		return
	}

	instance = NewClient(sdkKey)
}

// Options are an advanced options for configuring the Statsig SDK
type Options struct {
	API                   string      `json:"api"`
	Environment           Environment `json:"environment"`
	LocalMode             bool        `json:"localMode"`
	ConfigSyncInterval    time.Duration
	IDListSyncInterval    time.Duration
	LoggingInterval       time.Duration
	LoggingMaxBufferSize  int
	BootstrapValues       string
	RulesUpdatedCallback  func(rules string, time int64)
	InitTimeout           time.Duration
	DataAdapter           IDataAdapter
	OutputLoggerOptions   OutputLoggerOptions
	StatsigLoggerOptions  StatsigLoggerOptions
	EvaluationCallbacks   EvaluationCallbacks
	DisableCDN            bool // Disables use of CDN for downloading config specs
	UserPersistentStorage IUserPersistentStorage
}

type EvaluationCallbacks struct {
	GateEvaluationCallback       func(name string, result bool, exposure *ExposureEvent)
	ConfigEvaluationCallback     func(name string, result DynamicConfig, exposure *ExposureEvent)
	ExperimentEvaluationCallback func(name string, result DynamicConfig, exposure *ExposureEvent)
	LayerEvaluationCallback      func(name string, param string, result DynamicConfig, exposure *ExposureEvent)
}

type OutputLoggerOptions struct {
	LogCallback            func(message string, err error)
	EnableDebug            bool
	DisableInitDiagnostics bool
	DisableSyncDiagnostics bool
}

type StatsigLoggerOptions struct {
	DisableInitDiagnostics bool
	DisableSyncDiagnostics bool
	DisableApiDiagnostics  bool
	DisableAllLogging      bool
}

// Environment is a struct that represents the environment option
// See https://docs.statsig.com/guides/usingEnvironments
type Environment struct {
	Tier   string            `json:"tier"`
	Params map[string]string `json:"params"`
}

// IsInitialized returns whether the global Statsig instance has already been initialized or not
func IsInitialized() bool {
	return instance != nil
}

// InitializeWithOptions initializes the global Statsig instance with the given sdkKey and options
func InitializeWithOptions(sdkKey string, options *Options) {
	InitializeGlobalOutputLogger(options.OutputLoggerOptions)
	InitializeGlobalSessionID()
	if IsInitialized() {
		Logger().Log("Statsig is already initialized.", nil)
		return
	}

	if options.InitTimeout > 0 {
		channel := make(chan *Client, 1)
		go func() {
			client := NewClientWithOptions(sdkKey, options)
			channel <- client
		}()

		select {
		case res := <-channel:
			instance = res
		case <-time.After(options.InitTimeout):
			Logger().LogStep(StatsigProcessInitialize, "Timed out")
			return
		}
	} else {
		instance = NewClientWithOptions(sdkKey, options)
	}
}

// CheckGate checks the value of a Feature Gate for the given user
func CheckGate(user User, gate string) bool {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling CheckGate"))
	}
	return instance.CheckGate(user, gate)
}

// CheckGateWithExposureLoggingDisabled checks the value of a Feature Gate for the given user without logging an exposure event
func CheckGateWithExposureLoggingDisabled(user User, gate string) bool {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling CheckGateWithExposureLoggingDisabled"))
	}
	return instance.CheckGateWithExposureLoggingDisabled(user, gate)
}

// GetGate gets the Feature Gate for the given user
func GetGate(user User, gate string) FeatureGate {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetGate"))
	}
	return instance.GetGate(user, gate)
}

// GetGateWithExposureLoggingDisabled gets the Feature Gate for the given user without logging an exposure event
func GetGateWithExposureLoggingDisabled(user User, gate string) FeatureGate {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetGateWithExposureLoggingDisabled"))
	}
	return instance.GetGateWithExposureLoggingDisabled(user, gate)
}

// ManuallyLogGateExposure logs an exposure event for the gate
func ManuallyLogGateExposure(user User, config string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogGateExposure"))
	}
	instance.ManuallyLogGateExposure(user, config)
}

// GetConfig gets the DynamicConfig value for the given user
func GetConfig(user User, config string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetConfig"))
	}
	return instance.GetConfig(user, config)
}

// GetConfigWithExposureLoggingDisabled gets the DynamicConfig value for the given user without logging an exposure event
func GetConfigWithExposureLoggingDisabled(user User, config string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetConfigWithExposureLoggingDisabled"))
	}
	return instance.GetConfigWithExposureLoggingDisabled(user, config)
}

// ManuallyLogConfigExposure logs an exposure event for the dynamic config
func ManuallyLogConfigExposure(user User, config string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogConfigExposure"))
	}
	instance.ManuallyLogConfigExposure(user, config)
}

// OverrideGate overrides the value of a Feature Gate for the given user
func OverrideGate(gate string, val bool) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideGate"))
	}
	instance.OverrideGate(gate, val)
}

// OverrideConfig overrides the DynamicConfig value for the given user
func OverrideConfig(config string, val map[string]interface{}) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideConfig"))
	}
	instance.OverrideConfig(config, val)
}

// OverrideLayer overrides the Layer value for the given user
func OverrideLayer(layer string, val map[string]interface{}) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideLayer"))
	}
	instance.OverrideLayer(layer, val)
}

// GetExperiment gets the DynamicConfig value of an Experiment for the given user
func GetExperiment(user User, experiment string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperiment"))
	}
	return instance.GetExperiment(user, experiment)
}

// GetExperimentWithExposureLoggingDisabled gets the DynamicConfig value of an Experiment for the given user without logging an exposure event
func GetExperimentWithExposureLoggingDisabled(user User, experiment string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperimentWithExposureLoggingDisabled"))
	}
	return instance.GetExperimentWithExposureLoggingDisabled(user, experiment)
}

// GetExperimentWithOptions gets the DynamicConfig value of an Experiment for the given user with configurable options
func GetExperimentWithOptions(user User, experiment string, options *GetExperimentOptions) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperimentWithOptions"))
	}
	return instance.GetExperimentWithOptions(user, experiment, options)
}

// ManuallyLogExperimentExposure logs an exposure event for the experiment
func ManuallyLogExperimentExposure(user User, experiment string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogExperimentExposure"))
	}
	instance.ManuallyLogExperimentExposure(user, experiment)
}

// GetUserPersistedValues gets the PersistedValues for the given user
func GetUserPersistedValues(user User, idType string) UserPersistedValues {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetUserPersistedValues"))
	}
	return instance.GetUserPersistedValues(user, idType)
}

// GetLayer gets the Layer object for the given user
func GetLayer(user User, layer string) Layer {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetLayer"))
	}
	return instance.GetLayer(user, layer)
}

// GetLayerWithExposureLoggingDisabled gets the Layer object for the given user without logging an exposure event
func GetLayerWithExposureLoggingDisabled(user User, layer string) Layer {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetLayerWithExposureLoggingDisabled"))
	}
	return instance.GetLayerWithExposureLoggingDisabled(user, layer)
}

// ManuallyLogLayerParameterExposure logs an exposure event for the parameter in the given layer
func ManuallyLogLayerParameterExposure(user User, layer string, parameter string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogLayerParameterExposure"))
	}
	instance.ManuallyLogLayerParameterExposure(user, layer, parameter)
}

// LogEvent logs an event to the Statsig console
func LogEvent(event Event) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling LogEvent"))
	}
	instance.LogEvent(event)
}

// LogImmediate logs a slice of events to Statsig server immediately
func LogImmediate(events []Event) (*http.Response, error) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling LogImmediate"))
	}
	return instance.LogImmediate(events)
}

func GetClientInitializeResponse(user User) ClientInitializeResponse {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetClientInitializeResponse"))
	}
	return instance.GetClientInitializeResponse(user, "")
}

func GetClientInitializeResponseForTargetApp(user User, clientKey string) ClientInitializeResponse {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetClientInitializeResponseForTargetApp"))
	}
	return instance.GetClientInitializeResponse(user, clientKey)
}

// Shutdown cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func Shutdown() {
	if !IsInitialized() {
		return
	}
	instance.Shutdown()
}

// ShutdownAndDangerouslyClearInstance is used for test only so we can clear the shared instance. Not thread safe.
func ShutdownAndDangerouslyClearInstance() {
	Shutdown()
	instance = nil
}
