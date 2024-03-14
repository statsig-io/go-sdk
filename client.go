package statsig

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Client is an instance of a StatsigClient for interfacing with Statsig Feature Gates, Dynamic Configs, Experiments, and Event Logging
type Client struct {
	sdkKey        string
	evaluator     *evaluator
	logger        *logger
	transport     *transport
	errorBoundary *errorBoundary
	options       *Options
	diagnostics   *diagnostics
}

// NewClient initializes a Statsig Client with the given sdkKey
func NewClient(sdkKey string) *Client {
	return NewClientWithOptions(sdkKey, &Options{})
}

// NewClientWithOptions initializes a Statsig Client with the given sdkKey and options
func NewClientWithOptions(sdkKey string, options *Options) *Client {
	diagnostics := newDiagnostics(options)
	diagnostics.initialize().overall().start().mark()
	if len(options.API) == 0 {
		options.API = "https://statsigapi.net/v1"
	}
	errorBoundary := newErrorBoundary(sdkKey, options, diagnostics)
	if !options.LocalMode && !strings.HasPrefix(sdkKey, "secret") {
		err := errors.New(InvalidSDKKeyError)
		panic(err)
	}
	transport := newTransport(sdkKey, options)
	logger := newLogger(transport, options, diagnostics)
	evaluator := newEvaluator(transport, errorBoundary, options, diagnostics, sdkKey)
	diagnostics.initialize().overall().end().success(true).mark()
	return &Client{
		sdkKey:        sdkKey,
		evaluator:     evaluator,
		logger:        logger,
		transport:     transport,
		errorBoundary: errorBoundary,
		options:       options,
		diagnostics:   diagnostics,
	}
}

// CheckGate checks the value of a Feature Gate for the given user
func (c *Client) CheckGate(user User, gate string) bool {
	options := checkGateOptions{disableLogExposures: false}
	return c.checkGateImpl(user, gate, options).Value
}

// CheckGateWithExposureLoggingDisabled checks the value of a Feature Gate for the given user without logging an exposure event
func (c *Client) CheckGateWithExposureLoggingDisabled(user User, gate string) bool {
	options := checkGateOptions{disableLogExposures: true}
	return c.checkGateImpl(user, gate, options).Value
}

// GetGate gets the Feature Gate for the given user
func (c *Client) GetGate(user User, gate string) FeatureGate {
	options := checkGateOptions{disableLogExposures: false}
	return c.checkGateImpl(user, gate, options)
}

// GetGateWithExposureLoggingDisabled checks the value of a Feature Gate for the given user without logging an exposure event
func (c *Client) GetGateWithExposureLoggingDisabled(user User, gate string) FeatureGate {
	options := checkGateOptions{disableLogExposures: true}
	return c.checkGateImpl(user, gate, options)
}

// ManuallyLogGateExposure logs an exposure event for the dynamic config
func (c *Client) ManuallyLogGateExposure(user User, gate string) {
	c.errorBoundary.captureVoid(func() {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.checkGate(user, gate)
		context := &logContext{isManualExposure: true}
		c.logger.logGateExposure(user, gate, res.Pass, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
	})
}

// GetConfig gets the DynamicConfig value for the given user
func (c *Client) GetConfig(user User, config string) DynamicConfig {
	options := &getConfigOptions{disableLogExposures: false}
	context := getConfigImplContext{configOptions: options}
	return c.getConfigImpl(user, config, context)
}

// GetConfigWithExposureLoggingDisabled gets the DynamicConfig value for the given user without logging an exposure event
func (c *Client) GetConfigWithExposureLoggingDisabled(user User, config string) DynamicConfig {
	options := &getConfigOptions{disableLogExposures: true}
	context := getConfigImplContext{configOptions: options}
	return c.getConfigImpl(user, config, context)
}

// ManuallyLogConfigExposure logs an exposure event for the config
func (c *Client) ManuallyLogConfigExposure(user User, config string) {
	c.errorBoundary.captureVoid(func() {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.getConfig(user, config, nil)
		context := &logContext{isManualExposure: true}
		c.logger.logConfigExposure(user, config, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
	})
}

// GetExperiment gets the DynamicConfig value of an Experiment for the given user
func (c *Client) GetExperiment(user User, experiment string) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "", "", nil)
	}
	options := &GetExperimentOptions{DisableLogExposures: false}
	context := getConfigImplContext{experimentOptions: options}
	return c.getConfigImpl(user, experiment, context)
}

// GetExperimentWithExposureLoggingDisabled gets the DynamicConfig value of an Experiment for the given user without logging an exposure event
func (c *Client) GetExperimentWithExposureLoggingDisabled(user User, experiment string) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "", "", nil)
	}
	options := &GetExperimentOptions{DisableLogExposures: true}
	context := getConfigImplContext{experimentOptions: options}
	return c.getConfigImpl(user, experiment, context)
}

// GetExperimentWithOptions gets the DynamicConfig value of an Experiment for the given user with configurable options
func (c *Client) GetExperimentWithOptions(user User, experiment string, options *GetExperimentOptions) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "", "", nil)
	}
	context := getConfigImplContext{experimentOptions: options}
	return c.getConfigImpl(user, experiment, context)
}

// ManuallyLogExperimentExposure logs an exposure event for the experiment
func (c *Client) ManuallyLogExperimentExposure(user User, experiment string) {
	c.ManuallyLogConfigExposure(user, experiment)
}

// GetUserPersistedValues gets the persisted values for the given user
func (c *Client) GetUserPersistedValues(user User, idType string) UserPersistedValues {
	return c.errorBoundary.captureGetUserPersistedValues(func() UserPersistedValues {
		persistedValues := c.evaluator.persistentStorageUtils.getUserPersistedValues(user, idType)
		if persistedValues == nil {
			return make(UserPersistedValues)
		} else {
			return persistedValues
		}
	})
}

// GetLayer gets the Layer object for the given user
func (c *Client) GetLayer(user User, layer string) Layer {
	options := getLayerOptions{disableLogExposures: false}
	return c.getLayerImpl(user, layer, options)
}

// GetLayerWithExposureLoggingDisabled gets the Layer object for the given user without logging an exposure event
func (c *Client) GetLayerWithExposureLoggingDisabled(user User, layer string) Layer {
	options := getLayerOptions{disableLogExposures: true}
	return c.getLayerImpl(user, layer, options)
}

// ManuallyLogLayerParameterExposure logs an exposure event for the parameter in the given layer
func (c *Client) ManuallyLogLayerParameterExposure(user User, layer string, parameter string) {
	c.errorBoundary.captureVoid(func() {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.getLayer(user, layer)
		config := NewLayer(layer, res.ConfigValue.Value, res.ConfigValue.RuleID, res.ConfigValue.GroupName, nil).configBase
		context := &logContext{isManualExposure: true}
		c.logger.logLayerExposure(user, config, parameter, *res, res.EvaluationDetails, context)
	})
}

// LogEvent logs an event to Statsig for analysis in the Statsig Console
func (c *Client) LogEvent(event Event) {
	c.errorBoundary.captureVoid(func() {
		event.User = normalizeUser(event.User, *c.options)
		if event.EventName == "" {
			return
		}
		c.logger.logCustom(event)
	})
}

// OverrideGate overrides the value of a Feature Gate for the given user
func (c *Client) OverrideGate(gate string, val bool) {
	c.errorBoundary.captureVoid(func() { c.evaluator.OverrideGate(gate, val) })
}

// OverrideConfig overrides the DynamicConfig value for the given user
func (c *Client) OverrideConfig(config string, val map[string]interface{}) {
	c.errorBoundary.captureVoid(func() { c.evaluator.OverrideConfig(config, val) })
}

// OverrideLayer overrides the Layer value for the given user
func (c *Client) OverrideLayer(layer string, val map[string]interface{}) {
	c.errorBoundary.captureVoid(func() { c.evaluator.OverrideLayer(layer, val) })
}

// LogImmediate logs a batch of events to Statsig for analysis in the Statsig Console
func (c *Client) LogImmediate(events []Event) (*http.Response, error) {
	if len(events) > 500 {
		err := errors.New(EventBatchSizeError)
		return nil, fmt.Errorf(err.Error())
	}
	eventsProcessed := make([]interface{}, 0)
	for _, event := range events {
		event.User = normalizeUser(event.User, *c.options)
		eventsProcessed = append(eventsProcessed, event)
	}
	input := logEventInput{
		Events:          eventsProcessed,
		StatsigMetadata: c.transport.metadata,
	}
	return c.transport.post("/log_event", input, nil, RequestOptions{})
}

// GetClientInitializeResponse gets the ClientInitializeResponse for the given user
func (c *Client) GetClientInitializeResponse(user User, clientKey string) ClientInitializeResponse {
	return c.errorBoundary.captureGetClientInitializeResponse(func() ClientInitializeResponse {
		if !c.verifyUser(user) {
			return *new(ClientInitializeResponse)
		}
		user = normalizeUser(user, *c.options)
		return c.evaluator.getClientInitializeResponse(user, clientKey)
	})
}

func (c *Client) verifyUser(user User) bool {
	if user.UserID == "" && len(user.CustomIDs) == 0 {
		err := errors.New(EmptyUserError)
		Logger().LogError(err)
		return false
	}
	return true
}

// Shutdown cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func (c *Client) Shutdown() {
	c.errorBoundary.captureVoid(func() {
		c.logger.flush(true)
		c.evaluator.shutdown()
	})
}

type checkGateOptions struct {
	disableLogExposures bool
}

type getConfigOptions struct {
	disableLogExposures bool
}

// GetExperimentOptions is a set of options for fetching an experiment
type GetExperimentOptions struct {
	DisableLogExposures bool
	PersistedValues     UserPersistedValues
}

type getLayerOptions struct {
	disableLogExposures bool
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

func (c *Client) checkGateImpl(user User, gate string, options checkGateOptions) FeatureGate {
	return c.errorBoundary.captureCheckGate(func() FeatureGate {
		if !c.verifyUser(user) {
			return *NewGate(gate, false, "", "")
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.checkGate(user, gate)
		if res.FetchFromServer {
			serverRes := fetchGate(user, gate, c.transport)
			res = &evalResult{Pass: serverRes.Value, RuleID: serverRes.RuleID}
		} else {
			var exposure *ExposureEvent = nil
			if !options.disableLogExposures {
				context := &logContext{isManualExposure: false}
				exposure = c.logger.logGateExposure(user, gate, res.Pass, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
			}
			if c.options.EvaluationCallbacks.GateEvaluationCallback != nil {
				c.options.EvaluationCallbacks.GateEvaluationCallback(gate, res.Pass, exposure)
			}
		}
		return *NewGate(gate, res.Pass, res.RuleID, res.GroupName)
	})
}

type getConfigImplContext struct {
	configOptions     *getConfigOptions
	experimentOptions *GetExperimentOptions
}

func (c *Client) getConfigImpl(user User, config string, context getConfigImplContext) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func() DynamicConfig {
		if !c.verifyUser(user) {
			return *NewConfig(config, nil, "", "", nil)
		}
		isExperiment := context.experimentOptions != nil
		var persistedValues UserPersistedValues
		if isExperiment {
			persistedValues = context.experimentOptions.PersistedValues
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.getConfig(user, config, persistedValues)
		if res.FetchFromServer {
			res = c.fetchConfigFromServer(user, config)
		} else {
			var exposure *ExposureEvent = nil
			var logExposure bool
			if isExperiment {
				logExposure = !context.experimentOptions.DisableLogExposures
			} else {
				logExposure = !context.configOptions.disableLogExposures
			}
			if logExposure {
				context := &logContext{isManualExposure: false}
				exposure = c.logger.logConfigExposure(user, config, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
			}
			if isExperiment && c.options.EvaluationCallbacks.ExperimentEvaluationCallback != nil {
				c.options.EvaluationCallbacks.ExperimentEvaluationCallback(config, res.ConfigValue, exposure)
			} else if c.options.EvaluationCallbacks.ConfigEvaluationCallback != nil {
				c.options.EvaluationCallbacks.ConfigEvaluationCallback(config, res.ConfigValue, exposure)
			}
		}
		return res.ConfigValue
	})
}

func (c *Client) getLayerImpl(user User, layer string, options getLayerOptions) Layer {
	return c.errorBoundary.captureGetLayer(func() Layer {
		if !c.verifyUser(user) {
			return *NewLayer(layer, nil, "", "", nil)
		}

		user = normalizeUser(user, *c.options)
		res := c.evaluator.getLayer(user, layer)

		if res.FetchFromServer {
			res = c.fetchConfigFromServer(user, layer)
		}

		logFunc := func(config configBase, parameterName string) {
			var exposure *ExposureEvent = nil
			if !options.disableLogExposures {
				context := &logContext{isManualExposure: false}
				exposure = c.logger.logLayerExposure(user, config, parameterName, *res, res.EvaluationDetails, context)
			}
			if c.options.EvaluationCallbacks.LayerEvaluationCallback != nil {
				c.options.EvaluationCallbacks.LayerEvaluationCallback(layer, parameterName, res.ConfigValue, exposure)
			}
		}

		return *NewLayer(layer, res.ConfigValue.Value, res.ConfigValue.RuleID, res.ConfigValue.GroupName, &logFunc)
	})
}

func fetchGate(user User, gateName string, t *transport) gateResponse {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: t.metadata,
	}
	var res gateResponse
	_, err := t.post("/check_gate", input, &res, RequestOptions{})
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
	_, err := t.post("/get_config", input, &res, RequestOptions{})
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
		ConfigValue: *NewConfig(configName, serverRes.Value, serverRes.RuleID, "", nil),
		RuleID:      serverRes.RuleID,
	}
}
