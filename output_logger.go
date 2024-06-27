package statsig

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
	switch errTyped := err.(type) {
	case string:
		o.Log(errTyped, nil)
	case error:
		o.Log("", errTyped)
	default:
		sanitized := sanitize(fmt.Sprintf("%+v", err))
		fmt.Fprintln(os.Stderr, sanitized)
	}
}

func (o *OutputLogger) isInitialized() bool {
	return o != nil
}

func sanitize(string string) string {
	keyPattern := regexp.MustCompile(`secret-[a-zA-Z0-9]+`)
	return keyPattern.ReplaceAllString(string, "secret-****")
}
