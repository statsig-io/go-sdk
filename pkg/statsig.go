package statsig

import (
	"statsig/internal/evaluation"
	"statsig/internal/logging"
	"statsig/internal/net"
	"statsig/pkg/types"
	"sync"
)

type Statsig struct {
	// TODO: fill this, add logger and etc.
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
		instance.evaluator = evaluation.New(sdkKey)
		instance.sdkKey = sdkKey
		instance.net = net.New(sdkKey, "https://api.statsig.com/v1/")
		instance.logger = logging.New(instance.net)
	})
}

func CheckGate(user types.StatsigUser, gate string) bool {
	res := instance.evaluator.CheckGate(user, gate)
	if res.FetchFromServer {
		return instance.net.CheckGate(user, gate)
	}
	return res.Pass
}

func GetConfig(user types.StatsigUser, config string) *types.DynamicConfig {
	res := instance.evaluator.GetConfig(user, config)
	if res.FetchFromServer {
		return instance.net.GetConfig(user, config)
	}
	return res.ConfigValue
}

func GetExperiment(user types.StatsigUser, experiment string) *types.DynamicConfig {
	res := instance.evaluator.GetConfig(user, experiment)
	if res.FetchFromServer {
		return instance.net.GetConfig(user, experiment)
	}
	return res.ConfigValue
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
