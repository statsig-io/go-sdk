package statsig

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// An instance of a StatsigClient for interfacing with Statsig Feature Gates, Dynamic Configs, Experiments, and Event Logging
type Client struct {
	sdkKey        string
	evaluator     *evaluator
	logger        *logger
	transport     *transport
	errorBoundary *errorBoundary
	options       *Options
	diagnostics   *diagnostics
}

// Initializes a Statsig Client with the given sdkKey
func NewClient(sdkKey string) *Client {
	return NewClientWithOptions(sdkKey, &Options{})
}

// Initializes a Statsig Client with the given sdkKey and options
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
	logger := newLogger(transport, options, diagnostics, errorBoundary)
	evaluator := newEvaluator(transport, errorBoundary, options, diagnostics, sdkKey)
	client := &Client{
		sdkKey:        sdkKey,
		evaluator:     evaluator,
		logger:        logger,
		transport:     transport,
		errorBoundary: errorBoundary,
		options:       options,
		diagnostics:   diagnostics,
	}

	if options.InitTimeout > 0 {
		channel := make(chan *Client, 1)
		go func() {
			client.init()
			channel <- client
		}()

		select {
		case res := <-channel:
			diagnostics.initialize().overall().end().success(true).mark()
			return res
		case <-time.After(options.InitTimeout):
			Logger().LogStep(StatsigProcessInitialize, "Timed out")
			diagnostics.initialize().overall().end().success(false).reason("timeout").mark()
			client.initInBackground()
			return client
		}
	} else {
		client.init()
	}

	diagnostics.initialize().overall().end().success(true).mark()
	return client
}

func (c *Client) init() {
	c.evaluator.initialize()
}

func (c *Client) initInBackground() {
	c.evaluator.store.startPolling()
}

// Checks the value of a Feature Gate for the given user
func (c *Client) CheckGate(user User, gate string) bool {
	return c.errorBoundary.captureCheckGate(func(context *evalContext) FeatureGate {
		return c.checkGateImpl(user, gate, context)
	}, &evalContext{Caller: "checkGate", ConfigName: gate}).Value
}

// Checks the value of a Feature Gate for the given user without logging an exposure event
func (c *Client) CheckGateWithExposureLoggingDisabled(user User, gate string) bool {
	return c.errorBoundary.captureCheckGate(func(context *evalContext) FeatureGate {
		return c.checkGateImpl(user, gate, context)
	}, &evalContext{Caller: "checkGateWithExposureLoggingDisabled", ConfigName: gate, DisableLogExposures: true}).Value
}

// Get the Feature Gate for the given user
func (c *Client) GetGate(user User, gate string) FeatureGate {
	return c.errorBoundary.captureCheckGate(func(context *evalContext) FeatureGate {
		return c.checkGateImpl(user, gate, context)
	}, &evalContext{Caller: "getGate", ConfigName: gate})
}

// Checks the value of a Feature Gate for the given user without logging an exposure event
func (c *Client) GetGateWithExposureLoggingDisabled(user User, gate string) FeatureGate {
	return c.errorBoundary.captureCheckGate(func(context *evalContext) FeatureGate {
		return c.checkGateImpl(user, gate, context)
	}, &evalContext{Caller: "getGateWithExposureLoggingDisabled", ConfigName: gate, DisableLogExposures: true})
}

// Logs an exposure event for the dynamic config
func (c *Client) ManuallyLogGateExposure(user User, gate string) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalGate(user, gate, context)
		c.logger.logGateExposure(user, gate, res, context)
	}, &evalContext{Caller: "logGateExposure", ConfigName: gate, IsManualExposure: true})
}

// Gets the DynamicConfig value for the given user
func (c *Client) GetConfig(user User, config string) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func(context *evalContext) DynamicConfig {
		return c.getConfigImpl(user, config, context)
	}, &evalContext{Caller: "getConfig", ConfigName: config})
}

// Gets the DynamicConfig value for the given user without logging an exposure event
func (c *Client) GetConfigWithExposureLoggingDisabled(user User, config string) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func(context *evalContext) DynamicConfig {
		return c.getConfigImpl(user, config, context)
	}, &evalContext{Caller: "getConfigWithExposureLoggingDisabled", ConfigName: config, DisableLogExposures: true})
}

// Logs an exposure event for the config
func (c *Client) ManuallyLogConfigExposure(user User, config string) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalConfig(user, config, context)
		c.logger.logConfigExposure(user, config, res, context)
	}, &evalContext{Caller: "logConfigExposure", ConfigName: config, IsManualExposure: true})
}

// Gets the layer name of an Experiment
func (c *Client) GetExperimentLayer(experiment string) (string, bool) {
	return c.errorBoundary.captureGetExperimentLayer(func(context *evalContext) (string, bool) {
		return c.evaluator.store.getExperimentLayer(experiment)
	}, &evalContext{Caller: "getExperimentLayer", ConfigName: experiment})
}

// Gets the DynamicConfig value of an Experiment for the given user
func (c *Client) GetExperiment(user User, experiment string) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func(context *evalContext) DynamicConfig {
		return c.getConfigImpl(user, experiment, context)
	}, &evalContext{Caller: "getExperiment", ConfigName: experiment, IsExperiment: true})
}

// Gets the DynamicConfig value of an Experiment for the given user without logging an exposure event
func (c *Client) GetExperimentWithExposureLoggingDisabled(user User, experiment string) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func(context *evalContext) DynamicConfig {
		return c.getConfigImpl(user, experiment, context)
	}, &evalContext{Caller: "getExperimentWithExposureLoggingDisabled", ConfigName: experiment, IsExperiment: true, DisableLogExposures: true})
}

// Gets the DynamicConfig value of an Experiment for the given user with configurable options
func (c *Client) GetExperimentWithOptions(user User, experiment string, options *GetExperimentOptions) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func(context *evalContext) DynamicConfig {
		return c.getConfigImpl(user, experiment, context)
	}, &evalContext{
		Caller:              "getExperimentWithOptions",
		ConfigName:          experiment,
		IsExperiment:        true,
		DisableLogExposures: options.DisableLogExposures,
		PersistedValues:     options.PersistedValues,
	})
}

// Logs an exposure event for the experiment
func (c *Client) ManuallyLogExperimentExposure(user User, experiment string) {
	c.ManuallyLogConfigExposure(user, experiment)
}

func (c *Client) GetUserPersistedValues(user User, idType string) UserPersistedValues {
	return c.errorBoundary.captureGetUserPersistedValues(func(context *errorContext) UserPersistedValues {
		persistedValues := c.evaluator.persistentStorageUtils.load(user, idType)
		if persistedValues == nil {
			return make(UserPersistedValues)
		} else {
			return persistedValues
		}
	}, &errorContext{Caller: "GetUserPersistedValues"})
}

// Gets the Layer object for the given user
func (c *Client) GetLayer(user User, layer string) Layer {
	return c.errorBoundary.captureGetLayer(func(context *evalContext) Layer {
		return c.getLayerImpl(user, layer, context)
	}, &evalContext{Caller: "getLayer", ConfigName: layer})
}

// Gets the Layer object for the given user without logging an exposure event
func (c *Client) GetLayerWithExposureLoggingDisabled(user User, layer string) Layer {
	return c.errorBoundary.captureGetLayer(func(context *evalContext) Layer {
		return c.getLayerImpl(user, layer, context)
	}, &evalContext{Caller: "getLayerWithExposureLoggingDisabled", ConfigName: layer, DisableLogExposures: true})
}

// Gets the Layer object for the given user with configurable options
func (c *Client) GetLayerWithOptions(user User, layer string, options *GetLayerOptions) Layer {
	return c.errorBoundary.captureGetLayer(func(context *evalContext) Layer {
		return c.getLayerImpl(user, layer, context)
	}, &evalContext{
		Caller:              "getLayerWithOptions",
		ConfigName:          layer,
		DisableLogExposures: options.DisableLogExposures,
		PersistedValues:     options.PersistedValues,
	})
}

// Logs an exposure event for the parameter in the given layer
func (c *Client) ManuallyLogLayerParameterExposure(user User, layer string, parameter string) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalLayer(user, layer, context)
		config := NewLayer(layer, res.JsonValue, res.RuleID, res.GroupName, nil, res.ConfigDelegate)
		c.logger.logLayerExposure(user, *config, parameter, res, context)
	}, &evalContext{Caller: "logLayerParameterExposure", ConfigName: layer, IsManualExposure: true})
}

// Logs an event to Statsig for analysis in the Statsig Console
func (c *Client) LogEvent(event Event) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		event.User = normalizeUser(event.User, *c.options)
		if event.EventName == "" {
			return
		}
		c.logger.logCustom(event)
	}, &evalContext{Caller: "logEvent"})
}

// Override the value of a Feature Gate for the given user
func (c *Client) OverrideGate(gate string, val bool) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		c.evaluator.OverrideGate(gate, val)
	}, &evalContext{Caller: "overrideGate", ConfigName: gate})
}

// Override the DynamicConfig value for the given user
func (c *Client) OverrideConfig(config string, val map[string]interface{}) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		c.evaluator.OverrideConfig(config, val)
	}, &evalContext{Caller: "overrideConfig", ConfigName: config})
}

// Override the Layer value for the given user
func (c *Client) OverrideLayer(layer string, val map[string]interface{}) {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		c.evaluator.OverrideLayer(layer, val)
	}, &evalContext{Caller: "overrideLayer", ConfigName: layer})
}

func (c *Client) LogImmediate(events []Event) (*http.Response, error) {
	if len(events) > 500 {
		err := errors.New(EventBatchSizeError)
		return nil, err
	}
	events_processed := make([]interface{}, 0)
	for _, event := range events {
		event.User = normalizeUser(event.User, *c.options)
		events_processed = append(events_processed, event)
	}
	return c.transport.log_event(events_processed, nil, RequestOptions{})
}

func (c *Client) GetClientInitializeResponse(user User, clientKey string, includeLocalOverrides bool) ClientInitializeResponse {
	options := &GCIROptions{
		IncludeLocalOverrides: includeLocalOverrides,
		ClientKey:             clientKey,
		HashAlgorithm:         "sha256",
	}
	return c.GetClientInitializeResponseImpl(user, options)
}

func (c *Client) GetClientInitializeResponseWithOptions(user User, options *GCIROptions) ClientInitializeResponse {
	return c.GetClientInitializeResponseImpl(user, options)
}

func (c *Client) GetClientInitializeResponseImpl(user User, options *GCIROptions) ClientInitializeResponse {
	return c.errorBoundary.captureGetClientInitializeResponse(func(context *evalContext) ClientInitializeResponse {
		if !c.verifyUser(user) {
			return *new(ClientInitializeResponse)
		}
		user = normalizeUser(user, *c.options)
		response := c.evaluator.getClientInitializeResponse(user, context)
		if response.Time == 0 {
			c.errorBoundary.logExceptionWithContext(
				errors.New("empty response from server"),
				errorContext{evalContext: context},
			)
		}
		return response
	}, &evalContext{
		Caller:                "getClientInitializeResponse",
		IncludeLocalOverrides: options.IncludeLocalOverrides,
		ClientKey:             options.ClientKey,
		TargetAppID:           options.TargetAppID,
		Hash:                  options.HashAlgorithm,
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

// Cleans up Statsig, persisting any Event Logs and cleanup processes
// Using any method is undefined after Shutdown() has been called
func (c *Client) Shutdown() {
	c.errorBoundary.captureVoid(func(context *evalContext) {
		c.logger.flush(true)
		c.evaluator.shutdown()
	}, &evalContext{Caller: "shutdown"})
}

type GetExperimentOptions struct {
	DisableLogExposures bool
	PersistedValues     UserPersistedValues
}

type GetLayerOptions struct {
	DisableLogExposures bool
	PersistedValues     UserPersistedValues
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

func (c *Client) checkGateImpl(user User, name string, context *evalContext) FeatureGate {
	if !c.verifyUser(user) {
		return *NewGate(name, false, "", "", nil)
	}
	user = normalizeUser(user, *c.options)
	res := c.evaluator.evalGate(user, name, context)
	if res.FetchFromServer {
		serverRes := fetchGate(user, name, c.transport)
		res = &evalResult{Value: serverRes.Value, RuleID: serverRes.RuleID}
	} else {
		exposure := c.logger.getGateExposureWithEvaluationDetails(user, name, res, context)
		if !context.DisableLogExposures {
			c.logger.logExposure(*exposure)
		}

		if c.options.EvaluationCallbacks.GateEvaluationCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.GateEvaluationCallback(name, res.Value, exposure)
			} else {
				c.options.EvaluationCallbacks.GateEvaluationCallback(name, res.Value, nil)
			}
		}

		if c.options.EvaluationCallbacks.ExposureCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.ExposureCallback(name, exposure)
			} else {
				c.options.EvaluationCallbacks.ExposureCallback(name, nil)
			}
		}
	}
	return *NewGate(name, res.Value, res.RuleID, res.GroupName, res.EvaluationDetails)
}

func (c *Client) getConfigImpl(user User, name string, context *evalContext) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(name, nil, "", "", nil)
	}
	user = normalizeUser(user, *c.options)
	res := c.evaluator.evalConfig(user, name, context)
	config := *NewConfig(name, res.JsonValue, res.RuleID, res.GroupName, res.EvaluationDetails)
	if res.FetchFromServer {
		res = c.fetchConfigFromServer(user, name)
		config = *NewConfig(name, res.JsonValue, res.RuleID, res.GroupName, res.EvaluationDetails)
	} else {
		exposure := c.logger.getConfigExposureWithEvaluationDetails(user, name, res, context)
		if !context.DisableLogExposures {
			c.logger.logExposure(*exposure)
		}

		if context.IsExperiment && c.options.EvaluationCallbacks.ExperimentEvaluationCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.ExperimentEvaluationCallback(name, config, exposure)
			} else {
				c.options.EvaluationCallbacks.ExperimentEvaluationCallback(name, config, nil)
			}
		} else if c.options.EvaluationCallbacks.ConfigEvaluationCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.ConfigEvaluationCallback(name, config, exposure)
			} else {
				c.options.EvaluationCallbacks.ConfigEvaluationCallback(name, config, nil)
			}
		}

		if c.options.EvaluationCallbacks.ExposureCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.ExposureCallback(name, exposure)
			} else {
				c.options.EvaluationCallbacks.ExposureCallback(name, nil)
			}
		}
	}
	return config
}

func (c *Client) getLayerImpl(user User, name string, context *evalContext) Layer {
	if !c.verifyUser(user) {
		return *NewLayer(name, nil, "", "", nil, "")
	}

	user = normalizeUser(user, *c.options)
	res := c.evaluator.evalLayer(user, name, context)

	if res.FetchFromServer {
		res = c.fetchConfigFromServer(user, name)
	}

	logFunc := func(layer Layer, parameterName string) {
		exposure := c.logger.getLayerExposureWithEvaluationDetails(user, layer, parameterName, res, context)
		if !context.DisableLogExposures {
			c.logger.logExposure(*exposure)
		}
		if c.options.EvaluationCallbacks.LayerEvaluationCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.LayerEvaluationCallback(name, parameterName, DynamicConfig{layer.configBase}, exposure)
			} else {
				c.options.EvaluationCallbacks.LayerEvaluationCallback(name, parameterName, DynamicConfig{layer.configBase}, nil)
			}
		}
		if c.options.EvaluationCallbacks.ExposureCallback != nil {
			if c.options.EvaluationCallbacks.IncludeDisabledExposures || !context.DisableLogExposures {
				c.options.EvaluationCallbacks.ExposureCallback(name, exposure)
			} else {
				c.options.EvaluationCallbacks.ExposureCallback(name, nil)
			}
		}
	}

	return *NewLayer(name, res.JsonValue, res.RuleID, res.GroupName, &logFunc, res.ConfigDelegate)
}

func fetchGate(user User, gateName string, t *transport) gateResponse {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: t.metadata,
	}
	var res gateResponse
	_, err := t.post("/check_gate", input, &res, RequestOptions{}, nil)
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
	_, err := t.post("/get_config", input, &res, RequestOptions{}, nil)
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
		JsonValue: serverRes.Value,
		RuleID:    serverRes.RuleID,
	}
}
