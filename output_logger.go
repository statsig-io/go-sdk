package statsig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"time"
)

type StatsigProcess string

const (
	StatsigProcessInitialize StatsigProcess = "Initialize"
	StatsigProcessSync       StatsigProcess = "Sync"
	METRIC_PREFIX                           = "statsig.sdk"
)

var HIGH_CARDINALITY_TAGS = map[string]bool{
	"lcut":      true,
	"prev_lcut": true,
}

type OutputLogger struct {
	options             OutputLoggerOptions
	observabilityClient IObservabilityClient
}

func (o *OutputLogger) Log(msg string, err error) {
	if o.isInitialized() && o.options.LogCallback != nil {
		o.options.LogCallback(sanitize(msg), err)
	} else {
		timestamp := time.Now().Format(time.RFC3339)

		formatted := fmt.Sprintf("[%s][Statsig] %s", timestamp, msg)

		sanitized := ""
		if err != nil {
			formatted += err.Error()
			sanitized = sanitize(formatted)
			fmt.Fprintln(os.Stderr, sanitized)
		} else if msg != "" {
			sanitized = sanitize(formatted)
			fmt.Println(sanitized)
		}
	}
}

func (o *OutputLogger) Debug(any interface{}) {
	bytes, _ := json.MarshalIndent(any, "", "	")
	msg := fmt.Sprintf("%+v\n", string(bytes))
	o.Log(msg, nil)
}

func (o *OutputLogger) LogStep(process StatsigProcess, msg string) {
	if !o.isInitialized() || !o.options.EnableDebug {
		return
	}
	if o.options.DisableInitDiagnostics && process == StatsigProcessInitialize {
		return
	}
	if o.options.DisableSyncDiagnostics && process == StatsigProcessSync {
		return
	}
	o.Log(fmt.Sprintf("%s: %s", process, msg), nil)
}

func (o *OutputLogger) LogError(err interface{}) {
	var errMsg error
	switch e := err.(type) {
	case string:
		errMsg = errors.New(e)
	case error:
		errMsg = e
	default:
		errMsg = errors.New(convertToString(err))
	}

	o.Increment("sdk_exceptions_count", 1, map[string]interface{}{})
	stack := make([]byte, 1024)
	n := runtime.Stack(stack, false)
	o.Log(fmt.Sprintf("Error: %s\nStack Trace:\n%s", errMsg.Error(), string(stack[:n])), errMsg)
}

func (o *OutputLogger) Initialize() {
	if o.observabilityClient != nil {
		defer func() {
			if r := recover(); r != nil {
				o.Log("Observability client Init panicked", nil)
			}
		}()
		err := o.observabilityClient.Init(context.Background())
		if err != nil {
			o.Log("Observability client Init failed", err)
		}
	}
}

func (o *OutputLogger) Increment(metricName string, value int, tags map[string]interface{}) {
	if o.isInitialized() && o.observabilityClient != nil {
		defer func() {
			if r := recover(); r != nil {
				o.Log("Observability client Increment panicked", nil)
			}
		}()
		err := o.observabilityClient.Increment(fmt.Sprintf("%s.%s", METRIC_PREFIX, metricName), value, o.filterHighCardinalityTags(tags))
		if err != nil {
			o.Log("Observability client Increment failed", err)
		}
	}
}

func (o *OutputLogger) Gauge(metricName string, value float64, tags map[string]interface{}) {
	if o.isInitialized() && o.observabilityClient != nil {
		defer func() {
			if r := recover(); r != nil {
				o.Log("Observability client Gauge panicked", nil)
			}
		}()
		err := o.observabilityClient.Gauge(fmt.Sprintf("%s.%s", METRIC_PREFIX, metricName), value, o.filterHighCardinalityTags(tags))
		if err != nil {
			o.Log("Observability client Gauge failed", err)
		}
	}
}

func (o *OutputLogger) Distribution(metricName string, value float64, tags map[string]interface{}) {
	if o.isInitialized() && o.observabilityClient != nil {
		defer func() {
			if r := recover(); r != nil {
				o.Log("Observability client Distribution panicked", nil)
			}
		}()
		err := o.observabilityClient.Distribution(fmt.Sprintf("%s.%s", METRIC_PREFIX, metricName), value, o.filterHighCardinalityTags(tags))
		if err != nil {
			o.Log("Observability client Distribution failed", err)
		}
	}
}

func (o *OutputLogger) Shutdown() {
	if o.isInitialized() && o.observabilityClient != nil {
		defer func() {
			if r := recover(); r != nil {
				o.Log("Observability client Shutdown panicked", nil)
			}
		}()
		err := o.observabilityClient.Shutdown(context.Background())
		if err != nil {
			o.Log("Observability client Shutdown failed", err)
		}
	}
}

func (o *OutputLogger) LogPostInit(statsigOptions *Options, initDetails InitializeDetails) {
	if statsigOptions != nil && statsigOptions.LocalMode {
		if initDetails.Success {
			o.Log("Statsig SDK instance initialized in local mode. No data will be fetched from the Statsig servers.", nil)
		} else {
			o.Log("Statsig SDK instance failed to initialize in local mode.", nil)
		}
		return
	}

	o.Distribution("initialization", initDetails.Duration.Seconds(), map[string]interface{}{
		"source":          initDetails.Source.String(),
		"store_populated": initDetails.StorePopulated,
		"init_success":    initDetails.Success,
		"init_source_api": initDetails.SourceAPI,
	})

	if initDetails.Success {
		if initDetails.StorePopulated {
			message := fmt.Sprintf("Statsig SDK instance initialized successfully with data from %s", initDetails.Source)
			if initDetails.SourceAPI != "" {
				message += fmt.Sprintf("[%s]", initDetails.SourceAPI)
			}
			message += "."
			o.Log(message, nil)
		} else {
			o.Log("Statsig SDK instance initialized, but config store is not populated. The SDK is using default values for evaluation.", nil)
		}
	} else {
		if initDetails.Error != nil && initDetails.Error.Error() == "timed out" {
			o.Log("Statsig SDK instance initialization timed out.", nil)
		} else {
			o.Log("Statsig SDK instance Initialized failed!", nil)
		}
	}
}

func (o *OutputLogger) LogConfigSyncUpdate(initialized bool, hasUpdate bool, lcut int64, prevLcut int64, source string, api string) {
	if !initialized {
		return // do not log for initialize
	}
	if !hasUpdate {
		o.Increment("config_no_update", 1, map[string]interface{}{
			"source":     source,
			"source_api": api,
		})
		return
	}
	lcutDiff := prevLcut - lcut
	absLcutDiff := intAbs(lcutDiff)
	o.Distribution("config_propagation_diff", float64(absLcutDiff), map[string]interface{}{
		"source":     source,
		"source_api": api,
		"lcut":       lcut,
		"prev_lcut":  prevLcut,
	})
}

func (o *OutputLogger) isInitialized() bool {
	return o != nil
}

func sanitize(string string) string {
	keyPattern := regexp.MustCompile(`secret-[a-zA-Z0-9]+`)
	return keyPattern.ReplaceAllString(string, "secret-****")
}

func (o *OutputLogger) filterHighCardinalityTags(tags map[string]interface{}) map[string]interface{} {
	if !o.isInitialized() || o.observabilityClient == nil {
		return tags
	}

	filteredTags := make(map[string]interface{})
	for tag, value := range tags {
		defer func() {
			if r := recover(); r != nil {
				o.Log(fmt.Sprintf("Observability client ShouldEnableHighCardinalityForThisTag panicked: %v", r), nil)
			}
		}()
		if !HIGH_CARDINALITY_TAGS[tag] || o.observabilityClient.ShouldEnableHighCardinalityForThisTag(tag) {
			filteredTags[tag] = value
		}
	}
	return filteredTags
}
