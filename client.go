package statsig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// An instance of a StatsigClient for interfacing with Statsig Feature Gates, Dynamic Configs, Experiments, and Event Logging
type Client struct {
	sdkKey        string
	evaluator     *evaluator
	logger        *logger
	transport     *transport
	errorBoundary *errorBoundary
	options       *Options
}

// Initializes a Statsig Client with the given sdkKey
func NewClient(sdkKey string) *Client {
	return NewClientWithOptions(sdkKey, &Options{API: DefaultEndpoint})
}

// Initializes a Statsig Client with the given sdkKey and options
func NewClientWithOptions(sdkKey string, options *Options) *Client {
	if len(options.API) == 0 {
		options.API = "https://statsigapi.net/v1"
	}
	transport := newTransport(sdkKey, options)
	errorBoundary := newErrorBoundary()
	logger := newLogger(transport)
	evaluator := newEvaluator(transport, errorBoundary, options)
	if !options.LocalMode && !strings.HasPrefix(sdkKey, "secret") {
		err := errors.New(InvalidSDKKeyError)
		errorBoundary.logException(err)
		panic(err)
	}
	return &Client{
		sdkKey:        sdkKey,
		evaluator:     evaluator,
		logger:        logger,
		transport:     transport,
		errorBoundary: errorBoundary,
		options:       options,
	}
}

// Checks the value of a Feature Gate for the given user
func (c *Client) CheckGate(user User, gate string) bool {
	if !c.verifyUser(user) {
		return false
	}
	user = normalizeUser(user, *c.options)
	res := c.evaluator.checkGate(user, gate)
	if res.FetchFromServer {
		serverRes := fetchGate(user, gate, c.transport)
		res = &evalResult{Pass: serverRes.Value, Id: serverRes.RuleID}
	} else {
		c.logger.logGateExposure(user, gate, res.Pass, res.Id, res.SecondaryExposures)
	}
	return res.Pass
}

// Gets the DynamicConfig value for the given user
func (c *Client) GetConfig(user User, config string) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(config, nil, "")
	}
	user = normalizeUser(user, *c.options)
	res := c.evaluator.getConfig(user, config)
	if res.FetchFromServer {
		res = c.fetchConfigFromServer(user, config)
	} else {
		c.logger.logConfigExposure(user, config, res.Id, res.SecondaryExposures)
	}
	return res.ConfigValue
}

// Gets the DynamicConfig value of an Experiment for the given user
func (c *Client) GetExperiment(user User, experiment string) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "")
	}
	return c.GetConfig(user, experiment)
}

// Gets the Layer object for the given user
func (c *Client) GetLayer(user User, layer string) Layer {
	if !c.verifyUser(user) {
		return *NewLayer(layer, nil, "", nil)
	}

	user = normalizeUser(user, *c.options)
	res := c.evaluator.getLayer(user, layer)

	if res.FetchFromServer {
		res = c.fetchConfigFromServer(user, layer)
	}

	logFunc := func(config configBase, parameterName string) {
		c.logger.logLayerExposure(user, config, parameterName, *res)
	}

	return *NewLayer(layer, res.ConfigValue.Value, res.ConfigValue.RuleID, &logFunc)
}

// Logs an event to Statsig for analysis in the Statsig Console
func (c *Client) LogEvent(event Event) {
	event.User = normalizeUser(event.User, *c.options)
	if event.EventName == "" {
		return
	}
	c.logger.logCustom(event)
}

// Override the value of a Feature Gate for the given user
func (c *Client) OverrideGate(gate string, val bool) {
	c.evaluator.OverrideGate(gate, val)
}

// Override the DynamicConfig value for the given user
func (c *Client) OverrideConfig(config string, val map[string]interface{}) {
	c.evaluator.OverrideConfig(config, val)
}

func (c *Client) LogImmediate(events []Event) (*http.Response, error) {
	if len(events) > 500 {
		err := errors.New(EventBatchSizeError)
		return nil, fmt.Errorf(err.Error())
	}
	events_processed := make([]interface{}, 0)
	for _, event := range events {
		event.User = normalizeUser(event.User, *c.options)
		events_processed = append(events_processed, event)
	}
	input := &logEventInput{
		Events:          events_processed,
		StatsigMetadata: c.transport.metadata,
	}
	body, err := json.Marshal(input)

	if err != nil {
		return nil, err
	}
	return c.transport.doRequest("/log_event", body)
}

func (c *Client) verifyUser(user User) bool {
	if user.UserID == "" {
		err := errors.New(EmptyUserError)
		fmt.Println(err.Error())
		return false
	}
	return true
}

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func (c *Client) Shutdown() {
	c.logger.flush(true)
	c.evaluator.shutdown()
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
	GateName        string          `json:"gateName"`
	User            User            `json:"user"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type getConfigInput struct {
	ConfigName      string          `json:"configName"`
	User            User            `json:"user"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

func fetchGate(user User, gateName string, t *transport) gateResponse {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: t.metadata,
	}
	var res gateResponse
	err := t.postRequest("/check_gate", input, &res)
	if err != nil {
		return gateResponse{
			Name:   gateName,
			Value:  false,
			RuleID: "",
		}
	}
	return res
}

func fetchConfig(user User, configName string, t *transport) configResponse {
	input := &getConfigInput{
		ConfigName:      configName,
		User:            user,
		StatsigMetadata: t.metadata,
	}
	var res configResponse
	err := t.postRequest("/get_config", input, &res)
	if err != nil {
		return configResponse{
			Name:   configName,
			RuleID: "",
		}
	}
	return res
}

func normalizeUser(user User, options Options) User {
	env := make(map[string]string)
	// Copy to avoid data race. We modify the map below.
	for k, v := range options.Environment.Params {
		env[k] = v
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

func (c *Client) fetchConfigFromServer(user User, configName string) *evalResult {
	serverRes := fetchConfig(user, configName, c.transport)
	return &evalResult{
		ConfigValue: *NewConfig(configName, serverRes.Value, serverRes.RuleID),
		Id:          serverRes.RuleID,
	}
}
