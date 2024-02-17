package statsig

import (
	"encoding/json"
	"fmt"
	"time"
)

type StatsigProcess string

const (
	StatsigProcessInitialize StatsigProcess = "Initialize"
	StatsigProcessSync       StatsigProcess = "Sync"
)

type OutputLogger struct {
	options OutputLoggerOptions
}

func (o *OutputLogger) Log(msg string, err error) {
	if o.isInitialized() && o.options.LogCallback != nil {
		o.options.LogCallback(msg, err)
	} else {
		formatted := msg
		if err != nil {
			if formatted != "" {
				formatted += "\n"
			}
			formatted += err.Error()
		}
		if formatted != "" {
			fmt.Print(formatted)
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
	timestamp := time.Now().Format(time.RFC3339)
	o.Log(fmt.Sprintf("[%s][Statsig] %s: %s\n", timestamp, process, msg), nil)
}

func (o *OutputLogger) LogError(err interface{}) {
	switch errTyped := err.(type) {
	case string:
		o.Log(errTyped, nil)
	case error:
		o.Log("", errTyped)
	default:
		fmt.Print(err)
	}
}

func (o *OutputLogger) isInitialized() bool {
	return o != nil
}
