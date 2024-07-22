// Package statsig implements feature gating and a/b testing
package statsig

import (
	"fmt"
	"net/http"
	"time"
)

var instance *Client

// Initializes the global Statsig instance with the given sdkKey
func Initialize(sdkKey string) {
	InitializeGlobalOutputLogger(OutputLoggerOptions{})
	InitializeGlobalSessionID()
	if IsInitialized() {
		Logger().Log("Statsig is already initialized.", nil)
		return
	}

	instance = NewClient(sdkKey)
}

// Advanced options for configuring the Statsig SDK
type Options struct {
	API                   string       `json:"api"`
	APIOverrides          APIOverrides `json:"api_overrides"`
	Environment           Environment  `json:"environment"`
	LocalMode             bool         `json:"localMode"`
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
	IPCountryOptions      IPCountryOptions
	UAParserOptions       UAParserOptions
}

type APIOverrides struct {
	DownloadConfigSpecs string `json:"download_config_specs"`
	GetIDLists          string `json:"get_id_lists"`
	LogEvent            string `json:"log_event"`
}

type EvaluationCallbacks struct {
	GateEvaluationCallback       func(name string, result bool, exposure *ExposureEvent)
	ConfigEvaluationCallback     func(name string, result DynamicConfig, exposure *ExposureEvent)
	ExperimentEvaluationCallback func(name string, result DynamicConfig, exposure *ExposureEvent)
	LayerEvaluationCallback      func(name string, param string, result DynamicConfig, exposure *ExposureEvent)
	ExposureCallback             func(name string, exposure *ExposureEvent)
	IncludeDisabledExposures     bool
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

type IPCountryOptions struct {
	Disabled     bool // Fully disable IP to country lookup
	LazyLoad     bool // Load in background
	EnsureLoaded bool // Wait until loaded when needed
}

type UAParserOptions struct {
	Disabled     bool // Fully disable UA parser
	LazyLoad     bool // Load in background
	EnsureLoaded bool // Wait until loaded when needed
}

// See https://docs.statsig.com/guides/usingEnvironments
type Environment struct {
	Tier   string            `json:"tier"`
	Params map[string]string `json:"params"`
}

// options for getClientInitializeResponse
type GCIROptions struct {
	IncludeLocalOverrides bool
	ClientKey             string
}

// IsInitialized returns whether the global Statsig instance has already been initialized or not
func IsInitialized() bool {
	return instance != nil
}

// Initializes the global Statsig instance with the given sdkKey and options
func InitializeWithOptions(sdkKey string, options *Options) {
	InitializeGlobalOutputLogger(options.OutputLoggerOptions)
	InitializeGlobalSessionID()
	if IsInitialized() {
		Logger().Log("Statsig is already initialized.", nil)
		return
	}

	instance = NewClientWithOptions(sdkKey, options)
}

// Checks the value of a Feature Gate for the given user
func CheckGate(user User, gate string) bool {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling CheckGate"))
	}
	return instance.CheckGate(user, gate)
}

// Checks the value of a Feature Gate for the given user without logging an exposure event
func CheckGateWithExposureLoggingDisabled(user User, gate string) bool {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling CheckGateWithExposureLoggingDisabled"))
	}
	return instance.CheckGateWithExposureLoggingDisabled(user, gate)
}

// Get the Feature Gate for the given user
func GetGate(user User, gate string) FeatureGate {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetGate"))
	}
	return instance.GetGate(user, gate)
}

// Get the Feature Gate for the given user without logging an exposure event
func GetGateWithExposureLoggingDisabled(user User, gate string) FeatureGate {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetGateWithExposureLoggingDisabled"))
	}
	return instance.GetGateWithExposureLoggingDisabled(user, gate)
}

// Logs an exposure event for the gate
func ManuallyLogGateExposure(user User, config string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogGateExposure"))
	}
	instance.ManuallyLogGateExposure(user, config)
}

// Gets the DynamicConfig value for the given user
func GetConfig(user User, config string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetConfig"))
	}
	return instance.GetConfig(user, config)
}

// Gets the DynamicConfig value for the given user without logging an exposure event
func GetConfigWithExposureLoggingDisabled(user User, config string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetConfigWithExposureLoggingDisabled"))
	}
	return instance.GetConfigWithExposureLoggingDisabled(user, config)
}

// Logs an exposure event for the dynamic config
func ManuallyLogConfigExposure(user User, config string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogConfigExposure"))
	}
	instance.ManuallyLogConfigExposure(user, config)
}

// Override the value of a Feature Gate for the given user
func OverrideGate(gate string, val bool) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideGate"))
	}
	instance.OverrideGate(gate, val)
}

// Override the DynamicConfig value for the given user
func OverrideConfig(config string, val map[string]interface{}) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideConfig"))
	}
	instance.OverrideConfig(config, val)
}

// Override the Layer value for the given user
func OverrideLayer(layer string, val map[string]interface{}) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling OverrideLayer"))
	}
	instance.OverrideLayer(layer, val)
}

// Gets the name of layer an Experiment
func GetExperimentLayer(experiment string) (string, bool) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperimentLayer"))
	}
	return instance.GetExperimentLayer(experiment)
}

// Gets the DynamicConfig value of an Experiment for the given user
func GetExperiment(user User, experiment string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperiment"))
	}
	return instance.GetExperiment(user, experiment)
}

// Gets the DynamicConfig value of an Experiment for the given user without logging an exposure event
func GetExperimentWithExposureLoggingDisabled(user User, experiment string) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperimentWithExposureLoggingDisabled"))
	}
	return instance.GetExperimentWithExposureLoggingDisabled(user, experiment)
}

// Gets the DynamicConfig value of an Experiment for the given user with configurable options
func GetExperimentWithOptions(user User, experiment string, options *GetExperimentOptions) DynamicConfig {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetExperimentWithOptions"))
	}
	return instance.GetExperimentWithOptions(user, experiment, options)
}

// Logs an exposure event for the experiment
func ManuallyLogExperimentExposure(user User, experiment string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogExperimentExposure"))
	}
	instance.ManuallyLogExperimentExposure(user, experiment)
}

func GetUserPersistedValues(user User, idType string) UserPersistedValues {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetUserPersistedValues"))
	}
	return instance.GetUserPersistedValues(user, idType)
}

// Gets the Layer object for the given user
func GetLayer(user User, layer string) Layer {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetLayer"))
	}
	return instance.GetLayer(user, layer)
}

// Gets the Layer object for the given user without logging an exposure event
func GetLayerWithExposureLoggingDisabled(user User, layer string) Layer {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetLayerWithExposureLoggingDisabled"))
	}
	return instance.GetLayerWithExposureLoggingDisabled(user, layer)
}

// Gets the Layer object for the given user with configurable options
func GetLayerWithOptions(user User, layer string, options *GetLayerOptions) Layer {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetLayerWithOptions"))
	}
	return instance.getLayerImpl(user, layer, options)
}

// Logs an exposure event for the parameter in the given layer
func ManuallyLogLayerParameterExposure(user User, layer string, parameter string) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling ManuallyLogLayerParameterExposure"))
	}
	instance.ManuallyLogLayerParameterExposure(user, layer, parameter)
}

// Logs an event to the Statsig console
func LogEvent(event Event) {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling LogEvent"))
	}
	instance.LogEvent(event)
}

// Logs a slice of events to Statsig server immediately
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
	return instance.GetClientInitializeResponse(user, "", false)
}

func GetClientInitializeResponseWithOptions(user User, options *GCIROptions) ClientInitializeResponse {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetClientInitializeResponseWithOptions"))
	}
	return instance.GetClientInitializeResponseWithOptions(user, options)
}

func GetClientInitializeResponseForTargetApp(user User, clientKey string) ClientInitializeResponse {
	if !IsInitialized() {
		panic(fmt.Errorf("must Initialize() statsig before calling GetClientInitializeResponseForTargetApp"))
	}
	return instance.GetClientInitializeResponse(user, clientKey, false)
}

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func Shutdown() {
	if !IsInitialized() {
		return
	}
	instance.Shutdown()
}

// For test only so we can clear the shared instance. Not thread safe.
func ShutdownAndDangerouslyClearInstance() {
	Shutdown()
	instance = nil
}
