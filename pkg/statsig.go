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
		instance.evaluator = evaluation.New(instance.net)
	})
}

func CheckGate(user types.StatsigUser, gate string) bool {
	res := instance.evaluator.CheckGate(user, gate)
	if res.FetchFromServer {
		serverRes := fetchGate(user, gate)
		res = &evaluation.EvalResult{Pass: serverRes.Value, Id: serverRes.RuleID}
	}
	instance.logger.LogGateExposure(user, gate, res.Pass, res.Id)
	return res.Pass
}

func GetConfig(user types.StatsigUser, config string) *types.DynamicConfig {
	res := instance.evaluator.GetConfig(user, config)
	if res.FetchFromServer {
		serverRes := fetchConfig(user, config)
		res = &evaluation.EvalResult{
			ConfigValue: types.NewConfig(config, serverRes.Value, serverRes.RuleID),
			Id:          serverRes.RuleID}
	}
	instance.logger.LogConfigExposure(user, config, res.Id)
	return res.ConfigValue
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
	instance.evaluator.Stop()
}

type gateResponse struct {
	Name   string `json:"name"`
	Value  bool   `json:"value"`
	RuleID string `json:"rule_id"`
}

type configResponse struct {
	Name   string                 `json:"name"`
	Value  map[string]interface{} `json:"value"`
	RuleID string                 `json:"rule_id"`
}

type checkGateInput struct {
	GateName        string              `json:"gateName"`
	User            types.StatsigUser   `json:"user"`
	StatsigMetadata net.StatsigMetadata `json:"statsigMetadata"`
}

type getConfigInput struct {
	ConfigName      string              `json:"configName"`
	User            types.StatsigUser   `json:"user"`
	StatsigMetadata net.StatsigMetadata `json:"statsigMetadata"`
}

func fetchGate(user types.StatsigUser, gateName string) gateResponse {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: instance.net.GetStatsigMetadata(),
	}
	var res gateResponse
	err := instance.net.PostRequest("check_gate", input, &res)
	if err != nil {
		return gateResponse{
			Name:   gateName,
			Value:  false,
			RuleID: "",
		}
	}
	return res
}

func fetchConfig(user types.StatsigUser, configName string) configResponse {
	input := &getConfigInput{
		ConfigName:      configName,
		User:            user,
		StatsigMetadata: instance.net.GetStatsigMetadata(),
	}
	var res configResponse
	err := instance.net.PostRequest("get_config", input, &res)
	if err != nil {
		return configResponse{
			Name:   configName,
			RuleID: "",
		}
	}
	return res
}
