package statsig

import (
	"statsig/pkg/types"
	"sync"
)

var instance *Client
var once sync.Once

func Initialize(sdkKey string) {
	once.Do(func() {
		instance = NewClient(sdkKey)
	})
}

func InitializeWithOptions(sdkKey string, options *types.StatsigOptions) {
	once.Do(func() {
		instance = NewWithOptions(sdkKey, options)
	})
}

func CheckGate(user types.StatsigUser, gate string) bool {
	return instance.CheckGate(user, gate)
}

func GetConfig(user types.StatsigUser, config string) *types.DynamicConfig {
	return instance.GetConfig(user, config)
}

func GetExperiment(user types.StatsigUser, experiment string) *types.DynamicConfig {
	return instance.GetExperiment(user, experiment)
}

func LogEvent(event types.StatsigEvent) {
	instance.LogEvent(event)
}

func Shutdown() {
	instance.Shutdown()
}
