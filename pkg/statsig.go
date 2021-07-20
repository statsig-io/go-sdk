package statsig

import (
	"statsig/internal/evaluation"
	"statsig/internal/logging"
	"statsig/internal/net"
	"statsig/pkg/types"
	"sync"
)

type Statsig struct {
	sdkKey    string
	evaluator *evaluation.Evaluator
	logger    *logging.Logger
	net       *net.Net
}

var instance *Statsig
var once sync.Once

func Initialize(sdkKey string) {
	once.Do(func() {
		instance = new(Statsig)
		instance.sdkKey = sdkKey
		instance.net = net.New(sdkKey, "https://api.statsig.com/v1/")
		instance.logger = logging.New(instance.net)
		instance.evaluator = evaluation.New(instance.net, instance.logger)
	})
}

func CheckGate(user types.StatsigUser, gate string) bool {
	return instance.evaluator.CheckGate(user, gate)
}

func GetConfig(user types.StatsigUser, config string) *types.DynamicConfig {
	return instance.evaluator.GetConfig(user, config)
}

func GetExperiment(user types.StatsigUser, experiment string) *types.DynamicConfig {
	return GetConfig(user, experiment)
}

func LogEvent(event types.StatsigEvent) {
	if event.EventName == "" {
		return
	}
	instance.logger.Log(event)
}

func Shutdown() {
	instance.logger.Flush()
}
