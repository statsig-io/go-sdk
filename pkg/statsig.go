package statsig

import (
	"fmt"
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

func CheckGate(user types.StatsigUser, gate string) (bool, error) {
	if instance == nil {
		return false, fmt.Errorf("must Initialize() statsig before calling CheckGate")
	}
	return instance.CheckGate(user, gate), nil
}

func GetConfig(user types.StatsigUser, config string) (*types.DynamicConfig, error) {
	if instance == nil {
		return nil, fmt.Errorf("must Initialize() statsig before calling GetConfig")
	}
	return instance.GetConfig(user, config), nil
}

func GetExperiment(user types.StatsigUser, experiment string) (*types.DynamicConfig, error) {
	if instance == nil {
		return nil, fmt.Errorf("must Initialize() statsig before calling GetExperiment")
	}
	return instance.GetExperiment(user, experiment), nil
}

func LogEvent(event types.StatsigEvent) error {
	if instance == nil {
		return fmt.Errorf("must Initialize() statsig before calling LogEvent")
	}
	instance.LogEvent(event)
	return nil
}

func Shutdown() error {
	if instance == nil {
		return fmt.Errorf("must Initialize() statsig before calling LogEvent")
	}
	instance.Shutdown()
	return nil
}
