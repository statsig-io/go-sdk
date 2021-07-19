package statsig

import (
	"statsig/internal/evaluation"
	"statsig/internal/net"
	"statsig/pkg/types"
	"sync"
)

type Statsig struct {
	// TODO: fill this, add logger and etc.
	sdkKey      string
	evaluator   *evaluation.Evaluator
	net 		*net.Net
}

var instance *Statsig
var once sync.Once

func Initialize(sdkKey string) {
	once.Do(func() {
		instance = new(Statsig)
		instance.evaluator = evaluation.New(sdkKey)
		instance.sdkKey = sdkKey
		instance.net = net.New(sdkKey, "https://api.statsig.com/v1/")
	})
}

func CheckGate(user types.StatsigUser, gate string) bool {
	return instance.net.CheckGate(user, gate)
}

func GetConfig(user types.StatsigUser, config string) *types.DynamicConfig {
	return instance.net.GetConfig(user, config)
}

func GetExperiment(user types.StatsigUser, experiment string) *types.DynamicConfig {
	return instance.net.GetConfig(user, experiment)
}