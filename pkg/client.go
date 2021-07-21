package statsig

import (
	"statsig/internal/evaluation"
	"statsig/internal/logging"
	"statsig/internal/net"
	"statsig/pkg/types"
	"strings"
)

type Client struct {
	sdkKey    string
	evaluator *evaluation.Evaluator
	logger    *logging.Logger
	net       *net.Net
}

func NewClient(sdkKey string) *Client {
	return NewWithOptions(sdkKey, &types.StatsigOptions{API: "https://api.statsig.com/v1/"})
}

func NewWithOptions(sdkKey string, options *types.StatsigOptions) *Client {
	net := net.New(sdkKey, options.API)
	logger := logging.New(net)
	evaluator := evaluation.New(net)
	if !strings.HasPrefix(sdkKey, "secret") {
		panic("Must provide a valid SDK key.")
	}
	return &Client{sdkKey: sdkKey, evaluator: evaluator, logger: logger, net: net}
}

func (c *Client) CheckGate(user types.StatsigUser, gate string) bool {
	res := c.evaluator.CheckGate(user, gate)
	if res.FetchFromServer {
		serverRes := fetchGate(user, gate, c.net)
		res = &evaluation.EvalResult{Pass: serverRes.Value, Id: serverRes.RuleID}
	}
	c.logger.LogGateExposure(user, gate, res.Pass, res.Id)
	return res.Pass
}

func (c *Client) GetConfig(user types.StatsigUser, config string) *types.DynamicConfig {
	res := c.evaluator.GetConfig(user, config)
	if res.FetchFromServer {
		serverRes := fetchConfig(user, config, c.net)
		res = &evaluation.EvalResult{
			ConfigValue: *types.NewConfig(config, serverRes.Value, serverRes.RuleID),
			Id:          serverRes.RuleID}
	}
	c.logger.LogConfigExposure(user, config, res.Id)
	return &res.ConfigValue
}

func (c *Client) GetExperiment(user types.StatsigUser, experiment string) *types.DynamicConfig {
	return c.GetConfig(user, experiment)
}

func (c *Client) LogEvent(event types.StatsigEvent) {
	if event.EventName == "" {
		return
	}
	c.logger.Log(event)
}

func (c *Client) Shutdown() {
	c.logger.Flush()
	c.evaluator.Stop()
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

func fetchGate(user types.StatsigUser, gateName string, net *net.Net) gateResponse {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: net.GetStatsigMetadata(),
	}
	var res gateResponse
	err := net.PostRequest("check_gate", input, &res)
	if err != nil {
		return gateResponse{
			Name:   gateName,
			Value:  false,
			RuleID: "",
		}
	}
	return res
}

func fetchConfig(user types.StatsigUser, configName string, net *net.Net) configResponse {
	input := &getConfigInput{
		ConfigName:      configName,
		User:            user,
		StatsigMetadata: net.GetStatsigMetadata(),
	}
	var res configResponse
	err := net.PostRequest("get_config", input, &res)
	if err != nil {
		return configResponse{
			Name:   configName,
			RuleID: "",
		}
	}
	return res
}
