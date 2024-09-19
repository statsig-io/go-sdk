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
	options := checkGateOptions{disableLogExposures: false}
	context := StatsigContext{Caller: "checkGate", ConfigName: gate}
	return c.checkGateImpl(user, gate, options, context).Value
}

// Checks the value of a Feature Gate for the given user without logging an exposure event
func (c *Client) CheckGateWithExposureLoggingDisabled(user User, gate string) bool {
	options := checkGateOptions{disableLogExposures: true}
	context := StatsigContext{Caller: "checkGateWithExposureLoggingDisabled", ConfigName: gate}
	return c.checkGateImpl(user, gate, options, context).Value
}

// Get the Feature Gate for the given user
func (c *Client) GetGate(user User, gate string) FeatureGate {
	options := checkGateOptions{disableLogExposures: false}
	context := StatsigContext{Caller: "getGate", ConfigName: gate}
	return c.checkGateImpl(user, gate, options, context)
}

// Checks the value of a Feature Gate for the given user without logging an exposure event
func (c *Client) GetGateWithExposureLoggingDisabled(user User, gate string) FeatureGate {
	options := checkGateOptions{disableLogExposures: true}
	context := StatsigContext{Caller: "getGateWithExposureLoggingDisabled", ConfigName: gate}
	return c.checkGateImpl(user, gate, options, context)
}

// Logs an exposure event for the dynamic config
func (c *Client) ManuallyLogGateExposure(user User, gate string) {
	c.errorBoundary.captureVoid(func() {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalGate(user, gate, StatsigContext{Caller: "logGateExposure", ConfigName: gate})
		context := &logContext{isManualExposure: true}
		c.logger.logGateExposure(user, gate, res.Value, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
	})
}

// Gets the DynamicConfig value for the given user
func (c *Client) GetConfig(user User, config string) DynamicConfig {
	options := &getConfigOptions{disableLogExposures: false}
	context := getConfigImplContext{configOptions: options}
	statsigContext := StatsigContext{Caller: "getConfig", ConfigName: config}
	return c.getConfigImpl(user, config, context, statsigContext)
}

// Gets the DynamicConfig value for the given user without logging an exposure event
func (c *Client) GetConfigWithExposureLoggingDisabled(user User, config string) DynamicConfig {
	options := &getConfigOptions{disableLogExposures: true}
	context := getConfigImplContext{configOptions: options}
	statsigContext := StatsigContext{Caller: "getConfigWithExposureLoggingDisabled", ConfigName: config}
	return c.getConfigImpl(user, config, context, statsigContext)
}

// Logs an exposure event for the config
func (c *Client) ManuallyLogConfigExposure(user User, config string) {
	c.errorBoundary.captureVoid(func() {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalConfig(user, config, nil, StatsigContext{Caller: "logConfigExposure", ConfigName: config})
		context := &logContext{isManualExposure: true}
		c.logger.logConfigExposure(user, config, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
	})
}

// Gets the layer name of an Experiment
func (c *Client) GetExperimentLayer(experiment string) (string, bool) {
	return c.errorBoundary.captureGetExperimentLayer(func() (string, bool) {
		return c.evaluator.store.getExperimentLayer(experiment)
	})
}

// Gets the DynamicConfig value of an Experiment for the given user
func (c *Client) GetExperiment(user User, experiment string) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "", "", nil)
	}
	options := &GetExperimentOptions{DisableLogExposures: false}
	context := getConfigImplContext{experimentOptions: options}
	statsigContext := StatsigContext{Caller: "getExperiment", ConfigName: experiment}
	return c.getConfigImpl(user, experiment, context, statsigContext)
}

// Gets the DynamicConfig value of an Experiment for the given user without logging an exposure event
func (c *Client) GetExperimentWithExposureLoggingDisabled(user User, experiment string) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "", "", nil)
	}
	options := &GetExperimentOptions{DisableLogExposures: true}
	context := getConfigImplContext{experimentOptions: options}
	statsigContext := StatsigContext{Caller: "getExperimentWithExposureLoggingDisabled", ConfigName: experiment}
	return c.getConfigImpl(user, experiment, context, statsigContext)
}

// Gets the DynamicConfig value of an Experiment for the given user with configurable options
func (c *Client) GetExperimentWithOptions(user User, experiment string, options *GetExperimentOptions) DynamicConfig {
	if !c.verifyUser(user) {
		return *NewConfig(experiment, nil, "", "", nil)
	}
	context := getConfigImplContext{experimentOptions: options}
	statsigContext := StatsigContext{Caller: "getExperimentWithOptions", ConfigName: experiment}
	return c.getConfigImpl(user, experiment, context, statsigContext)
}

// Logs an exposure event for the experiment
func (c *Client) ManuallyLogExperimentExposure(user User, experiment string) {
	c.ManuallyLogConfigExposure(user, experiment)
}

func (c *Client) GetUserPersistedValues(user User, idType string) UserPersistedValues {
	return c.errorBoundary.captureGetUserPersistedValues(func() UserPersistedValues {
		persistedValues := c.evaluator.persistentStorageUtils.load(user, idType)
		if persistedValues == nil {
			return make(UserPersistedValues)
		} else {
			return persistedValues
		}
	})
}

// Gets the Layer object for the given user
func (c *Client) GetLayer(user User, layer string) Layer {
	options := &GetLayerOptions{DisableLogExposures: false, PersistedValues: nil}
	context := StatsigContext{Caller: "getLayer", ConfigName: layer}
	return c.getLayerImpl(user, layer, options, context)
}

// Gets the Layer object for the given user without logging an exposure event
func (c *Client) GetLayerWithExposureLoggingDisabled(user User, layer string) Layer {
	options := &GetLayerOptions{DisableLogExposures: true, PersistedValues: nil}
	context := StatsigContext{Caller: "getLayerWithExposureLoggingDisabled", ConfigName: layer}
	return c.getLayerImpl(user, layer, options, context)
}

// Gets the Layer object for the given user with configurable options
func (c *Client) GetLayerWithOptions(user User, layer string, options *GetLayerOptions) Layer {
	context := StatsigContext{Caller: "getLayerWithOptions", ConfigName: layer}
	return c.getLayerImpl(user, layer, options, context)
}

// Logs an exposure event for the parameter in the given layer
func (c *Client) ManuallyLogLayerParameterExposure(user User, layer string, parameter string) {
	c.errorBoundary.captureVoid(func() {
		if !c.verifyUser(user) {
			return
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalLayer(user, layer, nil, StatsigContext{Caller: "logLayerParameterExposure", ConfigName: layer})
		config := NewLayer(layer, res.JsonValue, res.RuleID, res.GroupName, nil, res.ConfigDelegate)
		context := &logContext{isManualExposure: true}
		c.logger.logLayerExposure(user, *config, parameter, *res, res.EvaluationDetails, context)
	})
}

// Logs an event to Statsig for analysis in the Statsig Console
func (c *Client) LogEvent(event Event) {
	c.errorBoundary.captureVoid(func() {
		event.User = normalizeUser(event.User, *c.options)
		if event.EventName == "" {
			return
		}
		c.logger.logCustom(event)
	})
}

// Override the value of a Feature Gate for all users
func (c *Client) OverrideGate(gate string, val bool) {
	c.errorBoundary.captureVoid(func() { c.evaluator.OverrideGate(gate, val) })
}

// Override the DynamicConfig value for all users
func (c *Client) OverrideConfig(config string, val map[string]interface{}) {
	c.errorBoundary.captureVoid(func() { c.evaluator.OverrideConfig(config, val) })
}

// Override the Layer value for all users
func (c *Client) OverrideLayer(layer string, val map[string]interface{}) {
	c.errorBoundary.captureVoid(func() { c.evaluator.OverrideLayer(layer, val) })
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
	return c.errorBoundary.captureGetClientInitializeResponse(func() ClientInitializeResponse {
		if !c.verifyUser(user) {
			return *new(ClientInitializeResponse)
		}
		user = normalizeUser(user, *c.options)
		response := c.evaluator.getClientInitializeResponse(user, options)
		if response.Time == 0 {
			c.errorBoundary.logException(errors.New("empty response from server"))
		}
		return response
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

func (c *Client) checkGateImpl(user User, name string, options checkGateOptions, context StatsigContext) FeatureGate {
	return c.errorBoundary.captureCheckGate(func() FeatureGate {
		if !c.verifyUser(user) {
			return *NewGate(name, false, "", "", nil)
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalGate(user, name, context)
		if res.FetchFromServer {
			serverRes := fetchGate(user, name, c.transport)
			res = &evalResult{Value: serverRes.Value, RuleID: serverRes.RuleID}
		} else {
			context := &logContext{isManualExposure: false}
			exposure := c.logger.getGateExposureWithEvaluationDetails(user, name, res.Value, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
			if !options.disableLogExposures {
				c.logger.logExposure(*exposure)
			}

			if c.options.EvaluationCallbacks.GateEvaluationCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || !options.disableLogExposures {
					c.options.EvaluationCallbacks.GateEvaluationCallback(name, res.Value, exposure)
				} else {
					c.options.EvaluationCallbacks.GateEvaluationCallback(name, res.Value, nil)
				}
			}

			if c.options.EvaluationCallbacks.ExposureCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || !options.disableLogExposures {
					c.options.EvaluationCallbacks.ExposureCallback(name, exposure)
				} else {
					c.options.EvaluationCallbacks.ExposureCallback(name, nil)
				}
			}
		}
		return *NewGate(name, res.Value, res.RuleID, res.GroupName, res.EvaluationDetails)
	})
}

type getConfigImplContext struct {
	configOptions     *getConfigOptions
	experimentOptions *GetExperimentOptions
}

func (c *Client) getConfigImpl(user User, name string, context getConfigImplContext, statsigContext StatsigContext) DynamicConfig {
	return c.errorBoundary.captureGetConfig(func() DynamicConfig {
		if !c.verifyUser(user) {
			return *NewConfig(name, nil, "", "", nil)
		}
		isExperiment := context.experimentOptions != nil
		var persistedValues UserPersistedValues
		if isExperiment {
			persistedValues = context.experimentOptions.PersistedValues
		}
		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalConfig(user, name, persistedValues, statsigContext)
		config := *NewConfig(name, res.JsonValue, res.RuleID, res.GroupName, res.EvaluationDetails)
		if res.FetchFromServer {
			res = c.fetchConfigFromServer(user, name)
			config = *NewConfig(name, res.JsonValue, res.RuleID, res.GroupName, res.EvaluationDetails)
		} else {
			var logExposure bool
			if isExperiment {
				logExposure = !context.experimentOptions.DisableLogExposures
			} else {
				logExposure = !context.configOptions.disableLogExposures
			}
			context := &logContext{isManualExposure: false}
			exposure := c.logger.getConfigExposureWithEvaluationDetails(user, name, res.RuleID, res.SecondaryExposures, res.EvaluationDetails, context)
			if logExposure {
				c.logger.logExposure(*exposure)
			}

			if isExperiment && c.options.EvaluationCallbacks.ExperimentEvaluationCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || logExposure {
					c.options.EvaluationCallbacks.ExperimentEvaluationCallback(name, config, exposure)
				} else {
					c.options.EvaluationCallbacks.ExperimentEvaluationCallback(name, config, nil)
				}
			} else if c.options.EvaluationCallbacks.ConfigEvaluationCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || logExposure {
					c.options.EvaluationCallbacks.ConfigEvaluationCallback(name, config, exposure)
				} else {
					c.options.EvaluationCallbacks.ConfigEvaluationCallback(name, config, nil)
				}
			}

			if c.options.EvaluationCallbacks.ExposureCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || logExposure {
					c.options.EvaluationCallbacks.ExposureCallback(name, exposure)
				} else {
					c.options.EvaluationCallbacks.ExposureCallback(name, nil)
				}
			}
		}
		return config
	})
}

func (c *Client) getLayerImpl(user User, name string, options *GetLayerOptions, context StatsigContext) Layer {
	return c.errorBoundary.captureGetLayer(func() Layer {
		if !c.verifyUser(user) {
			return *NewLayer(name, nil, "", "", nil, "")
		}

		user = normalizeUser(user, *c.options)
		res := c.evaluator.evalLayer(user, name, options.PersistedValues, context)

		if res.FetchFromServer {
			res = c.fetchConfigFromServer(user, name)
		}

		logFunc := func(layer Layer, parameterName string) {
			context := &logContext{isManualExposure: false}
			exposure := c.logger.getLayerExposureWithEvaluationDetails(user, layer, parameterName, *res, res.EvaluationDetails, context)
			if !options.DisableLogExposures {
				c.logger.logExposure(*exposure)
			}
			if c.options.EvaluationCallbacks.LayerEvaluationCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || !options.DisableLogExposures {
					c.options.EvaluationCallbacks.LayerEvaluationCallback(name, parameterName, DynamicConfig{layer.configBase}, exposure)
				} else {
					c.options.EvaluationCallbacks.LayerEvaluationCallback(name, parameterName, DynamicConfig{layer.configBase}, nil)
				}
			}
			if c.options.EvaluationCallbacks.ExposureCallback != nil {
				if c.options.EvaluationCallbacks.IncludeDisabledExposures || !options.DisableLogExposures {
					c.options.EvaluationCallbacks.ExposureCallback(name, exposure)
				} else {
					c.options.EvaluationCallbacks.ExposureCallback(name, nil)
				}
			}
		}

		return *NewLayer(name, res.JsonValue, res.RuleID, res.GroupName, &logFunc, res.ConfigDelegate)
	})
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
