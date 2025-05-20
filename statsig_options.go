package statsig

import (
	"net/http"
	"reflect"
	"time"
)

// Advanced options for configuring the Statsig SDK
type Options struct {
	API                   string       `json:"api"`
	APIOverrides          APIOverrides `json:"api_overrides"`
	FallbackToStatsigAPI  bool
	Transport             http.RoundTripper
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
	IPCountryOptions      IPCountryOptions
	UAParserOptions       UAParserOptions
	DisableIdList         bool
}

func (o *Options) GetSDKEnvironmentTier() string {
	if o.Environment.Tier != "" {
		return o.Environment.Tier
	}
	return "production"
}

func GetOptionLoggingCopy(options Options) map[string]interface{} {
	loggingCopy := make(map[string]interface{})
	val := reflect.ValueOf(options)

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		fieldValue := val.Field(i)
		switch fieldValue.Kind() {
		case reflect.String:
			if fieldValue.String() != "" {
				if fieldValue.Len() < 50 {
					loggingCopy[field.Name] = fieldValue.String()
				} else {
					loggingCopy[field.Name] = "set"
				}
			}

		case reflect.Bool:
			if fieldValue.Bool() {
				loggingCopy[field.Name] = true
			}

		case reflect.Int, reflect.Int64, reflect.Int32:
			if fieldValue.Int() != 0 {
				loggingCopy[field.Name] = fieldValue.Int()
			}

		case reflect.Float64, reflect.Float32:
			if fieldValue.Float() != 0 {
				loggingCopy[field.Name] = fieldValue.Float()
			}

		case reflect.Struct:
			if fieldValue.Type().Name() == "Duration" {
				loggingCopy[field.Name] = fieldValue.Interface().(time.Duration).String()
				break
			}
			if field.Name == "StatsigLoggerOptions" && !fieldValue.IsZero() {
				statsigLoggerOptions := map[string]interface{}{
					"DisableInitDiagnostics": fieldValue.Interface().(StatsigLoggerOptions).DisableInitDiagnostics,
					"DisableSyncDiagnostics": fieldValue.Interface().(StatsigLoggerOptions).DisableSyncDiagnostics,
					"DisableApiDiagnostics":  fieldValue.Interface().(StatsigLoggerOptions).DisableApiDiagnostics,
					"DisableAllLogging":      fieldValue.Interface().(StatsigLoggerOptions).DisableAllLogging,
				}
				loggingCopy[field.Name] = statsigLoggerOptions
				break
			}
			if field.Name == "APIOverrides" && !fieldValue.IsZero() {
				APIOptionOverrides := map[string]interface{}{
					"DownloadConfigSpecs": fieldValue.Interface().(APIOverrides).DownloadConfigSpecs,
					"GetIDLists":          fieldValue.Interface().(APIOverrides).GetIDLists,
					"LogEvent":            fieldValue.Interface().(APIOverrides).LogEvent,
				}
				loggingCopy[field.Name] = APIOptionOverrides
				break
			}
			if !fieldValue.IsZero() {
				loggingCopy[field.Name] = "set"
			}

		case reflect.Func:
			if !fieldValue.IsNil() {
				loggingCopy[field.Name] = "set"
			}

		case reflect.Interface:
			if !fieldValue.IsNil() {
				loggingCopy[field.Name] = "set"
			}

		default:
			// ignore other fields
		}
	}
	return loggingCopy
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
	TargetAppID           string
	HashAlgorithm         string
	IncludeConfigType     bool
	ConfigTypesToInclude  []ConfigType
	UseControlForUsersNotInExperiment bool
}

type ConfigType = string

const (
	FeatureGateType   ConfigType = "feature_gate"
	HoldoutType       ConfigType = "holdout"
	SegmentType       ConfigType = "segment"
	DynamicConfigType ConfigType = "dynamic_config"
	ExperimentType    ConfigType = "experiment"
	AutotuneType      ConfigType = "autotune"
	LayerType         ConfigType = "layer"
	UnknownType       ConfigType = "unknown"
)
