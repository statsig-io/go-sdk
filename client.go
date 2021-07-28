package statsig

import (
	"fmt"
	"strings"

	"github.com/statsig-io/go-sdk/internal/evaluation"
	"github.com/statsig-io/go-sdk/internal/logging"
	"github.com/statsig-io/go-sdk/internal/net"

	"github.com/statsig-io/go-sdk/types"
)

// An instance of a StatsigClient for interfacing with Statsig Feature Gates, Dynamic Configs, Experiments, and Event Logging
type Client struct {
	sdkKey    string
	evaluator *evaluation.Evaluator
	logger    *logging.Logger
	net       *net.Net
	options   *types.StatsigOptions
}

// Initializes a Statsig Client with the given sdkKey
func New(sdkKey string) *Client {
	return NewWithOptions(sdkKey, &types.StatsigOptions{API: "https://api.statsig.com/v1"})
}

// Initializes a Statsig Client with the given sdkKey and options
func NewWithOptions(sdkKey string, options *types.StatsigOptions) *Client {
	if len(options.API) == 0 {
		options.API = "https://api.statsig.com/v1"
	}
	net := net.New(sdkKey, options.API)
	logger := logging.New(net)
	evaluator := evaluation.New(net)
	if !strings.HasPrefix(sdkKey, "secret") {
		panic("Must provide a valid SDK key.")
	}
	return &Client{
		sdkKey:    sdkKey,
		evaluator: evaluator,
		logger:    logger,
		net:       net,
		options:   options,
	}
}

// Checks the value of a Feature Gate for the given user
func (c *Client) CheckGate(user types.StatsigUser, gate string) bool {
	if user.UserID == "" {
		fmt.Println("A non-empty StatsigUser.UserID is required. See https://docs.statsig.com/messages/serverRequiredUserID")
		return false
	}
	user = normalizeUser(user, *c.options)
	res := c.evaluator.CheckGate(user, gate)
	if res.FetchFromServer {
		serverRes := fetchGate(user, gate, c.net)
		res = &evaluation.EvalResult{Pass: serverRes.Value, Id: serverRes.RuleID}
	}
	c.logger.LogGateExposure(user, gate, res.Pass, res.Id)
	return res.Pass
}

// Gets the DynamicConfig value for the given user
func (c *Client) GetConfig(user types.StatsigUser, config string) types.DynamicConfig {
	if user.UserID == "" {
		fmt.Println("A non-empty StatsigUser.UserID is required. See https://docs.statsig.com/messages/serverRequiredUserID")
		return *types.NewConfig(config, nil, "")
	}
	user = normalizeUser(user, *c.options)
	res := c.evaluator.GetConfig(user, config)
	if res.FetchFromServer {
		serverRes := fetchConfig(user, config, c.net)
		res = &evaluation.EvalResult{
			ConfigValue: *types.NewConfig(config, serverRes.Value, serverRes.RuleID),
			Id:          serverRes.RuleID}
	}
	c.logger.LogConfigExposure(user, config, res.Id)
	return res.ConfigValue
}

// Gets the DynamicConfig value of an Experiment for the given user
func (c *Client) GetExperiment(user types.StatsigUser, experiment string) types.DynamicConfig {
	if user.UserID == "" {
		fmt.Println("A non-empty StatsigUser.UserID is required. See https://docs.statsig.com/messages/serverRequiredUserID")
		return *types.NewConfig(experiment, nil, "")
	}
	return c.GetConfig(user, experiment)
}

// Logs an event to Statsig for analysis in the Statsig Console
func (c *Client) LogEvent(event types.StatsigEvent) {
	event.User = normalizeUser(event.User, *c.options)
	if event.EventName == "" {
		return
	}
	c.logger.Log(event)
}

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func (c *Client) Shutdown() {
	c.logger.Flush(true)
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

func fetchGate(user types.StatsigUser, gateName string, n *net.Net) gateResponse {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: n.GetStatsigMetadata(),
	}
	var res gateResponse
	var err error
	go func() {
		err = n.PostRequest("/check_gate", input, &res)
	}()
	if err != nil {
		return gateResponse{
			Name:   gateName,
			Value:  false,
			RuleID: "",
		}
	}
	return res
}

func fetchConfig(user types.StatsigUser, configName string, n *net.Net) configResponse {
	input := &getConfigInput{
		ConfigName:      configName,
		User:            user,
		StatsigMetadata: n.GetStatsigMetadata(),
	}
	var res configResponse
	var err error
	go func() {
		err = n.PostRequest("/get_config", input, &res)
	}()
	if err != nil {
		return configResponse{
			Name:   configName,
			RuleID: "",
		}
	}
	return res
}

func normalizeUser(user types.StatsigUser, options types.StatsigOptions) types.StatsigUser {
	var env map[string]string
	if len(options.Environment.Params) > 0 {
		env = options.Environment.Params
	} else {
		env = make(map[string]string)
	}

	if options.Environment.Tier != "" {
		env["tier"] = options.Environment.Tier
	}
	for k, v := range user.StatsigEnvironment {
		env[k] = v
	}
	user.StatsigEnvironment = env
	return user
}
