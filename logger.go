package statsig

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ExposureEventName string

const (
	GateExposureEventName   ExposureEventName = "statsig::gate_exposure"
	ConfigExposureEventName ExposureEventName = "statsig::config_exposure"
	LayerExposureEventName  ExposureEventName = "statsig::layer_exposure"
)

type ExposureEvent struct {
	EventName          ExposureEventName      `json:"eventName"`
	User               User                   `json:"user"`
	Value              string                 `json:"value"`
	Metadata           map[string]string      `json:"metadata"`
	SecondaryExposures []SecondaryExposure    `json:"secondaryExposures"`
	Time               int64                  `json:"time"`
	StatsigMetadata    map[string]interface{} `json:"statsigMetadata"`
}

const diagnosticsEventName = "statsig::diagnostics"

type diagnosticsEvent struct {
	EventName string                 `json:"eventName"`
	Metadata  map[string]interface{} `json:"metadata"`
	Time      int64                  `json:"time"`
}

type logEventInput struct {
	Events          []interface{}   `json:"events"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type logEventResponse struct{}

type logger struct {
	events         []interface{}
	transport      *transport
	tick           *time.Ticker
	mu             sync.Mutex
	maxEvents      int
	disabled       bool
	diagnostics    *diagnostics
	options        *Options
	errorBoundary  *errorBoundary
	samplingKeySet *TTLSet
	SDKConfigs     *SDKConfigs
}

func newLogger(transport *transport, options *Options, diagnostics *diagnostics, errorBoundary *errorBoundary, sdkConfigs *SDKConfigs) *logger {
	loggingInterval := time.Minute
	maxEvents := 1000
	if options.LoggingInterval > 0 {
		loggingInterval = options.LoggingInterval
	}
	if options.LoggingMaxBufferSize > 0 {
		maxEvents = options.LoggingMaxBufferSize
	}
	disabled := options.StatsigLoggerOptions.DisableAllLogging
	log := &logger{
		events:         make([]interface{}, 0),
		transport:      transport,
		tick:           time.NewTicker(loggingInterval),
		maxEvents:      maxEvents,
		disabled:       disabled,
		diagnostics:    diagnostics,
		options:        options,
		errorBoundary:  errorBoundary,
		samplingKeySet: NewTTLSet(),
		SDKConfigs:     sdkConfigs,
	}

	go log.backgroundFlush()

	return log
}

func (l *logger) backgroundFlush() {
	for range l.tick.C {
		l.flush(false)
	}
}

func (l *logger) logCustom(evt Event) {
	evt.User.PrivateAttributes = nil
	if evt.Time == 0 {
		evt.Time = getUnixMilli()
	}
	l.logInternal(evt)
}

func (l *logger) logExposure(evt ExposureEvent) {
	evt.User.PrivateAttributes = nil
	if evt.Time == 0 {
		evt.Time = getUnixMilli()
	}
	l.logInternal(evt)
}

func (l *logger) logInternal(evt interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.disabled {
		return
	}

	l.events = append(l.events, evt)
	if len(l.events) >= l.maxEvents {
		l.flushInternal(false)
	}
}

func (l *logger) logGateExposure(
	user User,
	gateName string,
	res *evalResult,
	context *evalContext,
) *ExposureEvent {
	shouldLog, samplingRate, shadowLogged, samplingMode := l.determineSampling(EntityGate, gateName, res, &user, "", "")
	metadata := map[string]string{
		"gate":      gateName,
		"gateValue": strconv.FormatBool(res.Value),
		"ruleID":    res.RuleID,
	}
	if context != nil && context.IsManualExposure {
		metadata["isManualExposure"] = "true"
	}

	if res.ConfigVersion != nil {
		metadata["configVersion"] = strconv.Itoa(*res.ConfigVersion)
	}

	evt := &ExposureEvent{
		User:               user,
		EventName:          GateExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: res.SecondaryExposures,
	}
	l.addEvaluationDetailsToExposureEvent(evt, res.EvaluationDetails)
	l.addDeviceMetadataToExposureEvent(evt, res.DerivedDeviceMetadata)
	l.addSamplingMetadataToExposureEvent(evt, samplingRate, shadowLogged, samplingMode)
	if shouldLog && (context == nil || !context.DisableLogExposures) {
		l.logExposure(*evt)
	}
	return evt
}

func (l *logger) addEvaluationDetailsToExposureEvent(
	evt *ExposureEvent,
	evalDetails *EvaluationDetails,
) {
	if evalDetails != nil {
		evt.Metadata["reason"] = evalDetails.detailedReason()
		evt.Metadata["configSyncTime"] = strconv.FormatInt(evalDetails.ConfigSyncTime, 10)
		evt.Metadata["initTime"] = strconv.FormatInt(evalDetails.InitTime, 10)
		evt.Metadata["serverTime"] = strconv.FormatInt(evalDetails.ServerTime, 10)
	}
}

func (l *logger) addDeviceMetadataToExposureEvent(
	evt *ExposureEvent,
	deviceMetadata *DerivedDeviceMetadata,
) {
	if deviceMetadata != nil {
		evt.Metadata["os_name"] = deviceMetadata.OsName
		evt.Metadata["os_version"] = deviceMetadata.OsVersion
		evt.Metadata["browser_name"] = deviceMetadata.BrowserName
		evt.Metadata["browser_version"] = deviceMetadata.BrowserVersion
	}
}

func (l *logger) addSamplingMetadataToExposureEvent(
	evt *ExposureEvent,
	samplingRate *int,
	shadowLogged *string,
	samplingMode string,
) {
	tempMetadata := make(map[string]interface{})
	if samplingMode != "" {
		tempMetadata["samplingMode"] = samplingMode
	}
	if samplingRate != nil {
		tempMetadata["samplingRate"] = *samplingRate
	}
	if shadowLogged != nil {
		tempMetadata["shadowLogged"] = *shadowLogged
	}
	if len(tempMetadata) > 0 {
		if evt.StatsigMetadata == nil {
			evt.StatsigMetadata = tempMetadata
		}
	}
}

func (l *logger) logConfigExposure(
	user User,
	configName string,
	res *evalResult,
	context *evalContext,
) *ExposureEvent {
	shouldLog, samplingRate, shadowLogged, samplingMode := l.determineSampling(EntityConfig, configName, res, &user, "", "")
	metadata := map[string]string{
		"config":     configName,
		"ruleID":     res.RuleID,
		"rulePassed": strconv.FormatBool(res.Value),
	}
	if context != nil && context.IsManualExposure {
		metadata["isManualExposure"] = "true"
	}
	if res.ConfigVersion != nil {
		metadata["configVersion"] = strconv.Itoa(*res.ConfigVersion)
	}
	evt := &ExposureEvent{
		User:               user,
		EventName:          ConfigExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: res.SecondaryExposures,
	}
	l.addEvaluationDetailsToExposureEvent(evt, res.EvaluationDetails)
	l.addDeviceMetadataToExposureEvent(evt, res.DerivedDeviceMetadata)
	l.addSamplingMetadataToExposureEvent(evt, samplingRate, shadowLogged, samplingMode)
	if shouldLog && (context == nil || !context.DisableLogExposures) {
		l.logExposure(*evt)
	}
	return evt
}

func (l *logger) logLayerExposure(
	user User,
	config Layer,
	parameterName string,
	evalResult *evalResult,
	context *evalContext,
) *ExposureEvent {
	allocatedExperiment := ""
	exposures := evalResult.UndelegatedSecondaryExposures
	isExplicit := false
	for _, s := range evalResult.ExplicitParameters {
		if s == parameterName {
			isExplicit = true
		}
	}

	if isExplicit {
		allocatedExperiment = evalResult.ConfigDelegate
		exposures = evalResult.SecondaryExposures
	}
	metadata := map[string]string{
		"config":              config.Name,
		"ruleID":              config.RuleID,
		"allocatedExperiment": allocatedExperiment,
		"parameterName":       parameterName,
		"isExplicitParameter": strconv.FormatBool(isExplicit),
	}

	if evalResult.ConfigVersion != nil {
		metadata["configVersion"] = strconv.Itoa(*evalResult.ConfigVersion)
	}

	if context != nil && context.IsManualExposure {
		metadata["isManualExposure"] = "true"
	}

	evt := &ExposureEvent{
		User:               user,
		EventName:          LayerExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	shouldLog, samplingRate, shadowLogged, samplingMode := l.determineSampling(EntityLayer, config.Name, evalResult, &user, parameterName, allocatedExperiment)
	l.addEvaluationDetailsToExposureEvent(evt, evalResult.EvaluationDetails)
	l.addDeviceMetadataToExposureEvent(evt, evalResult.DerivedDeviceMetadata)
	l.addSamplingMetadataToExposureEvent(evt, samplingRate, shadowLogged, samplingMode)
	if shouldLog && (context == nil || !context.DisableLogExposures) {
		l.logExposure(*evt)
	}
	return evt
}

func (l *logger) shutdown() {
	l.samplingKeySet.Shutdown()
	l.flush(true)
}

func (l *logger) flush(closing bool) {
	l.logDiagnosticsEvents(l.diagnostics)
	l.mu.Lock()
	defer l.mu.Unlock()

	l.flushInternal(closing)
}

func (l *logger) flushInternal(closing bool) {
	if closing {
		l.tick.Stop()
	}
	if len(l.events) == 0 {
		return
	}

	if closing {
		l.sendEvents(l.events)
	} else {
		go l.sendEvents(l.events)
	}

	l.events = make([]interface{}, 0)
}

func (l *logger) sendEvents(events []interface{}) {
	var res logEventResponse
	_, err := l.transport.log_event(events, &res, RequestOptions{retries: maxRetries})
	if err != nil {
		context := errorContext{
			Caller:       "statsig::log_event_failed",
			EventCount:   len(events),
			BypassDedupe: true,
			LogToOutput:  true,
		}
		err := &LogEventError{
			Events: len(events),
			Err:    err,
		}
		l.errorBoundary.logExceptionWithContext(err, context)
	}
}

func (l *logger) logDiagnosticsEvents(d *diagnostics) {
	l.logDiagnosticsEvent(d.initDiagnostics)
	l.logDiagnosticsEvent(d.syncDiagnostics)
	l.logDiagnosticsEvent(d.apiDiagnostics)
}

func (l *logger) logDiagnosticsEvent(d *diagnosticsBase) {
	if d.isDisabled() {
		return
	}
	serialized, shouldSample := d.serializeWithSampling()
	markers, exists := serialized["markers"]
	if !shouldSample || !exists {
		return
	}
	markersTyped, ok := markers.([]marker)
	d.clearMarkers()
	if !ok || len(markersTyped) == 0 {
		return
	}
	event := diagnosticsEvent{
		EventName: diagnosticsEventName,
		Time:      getUnixMilli(),
		Metadata:  serialized,
	}
	l.logInternal(event)
}

func (l *logger) determineSampling(entityType EntityType,
	name string,
	result *evalResult,
	user *User,
	paramName string,
	allocatedExperiment string,
) (shouldLog bool, loggedSamplingRate *int, shadowLogged *string, samplingMode string) {
	defer func() {
		if r := recover(); r != nil {
			l.errorBoundary.logException(errors.New("determineSampling panicked"))
			shouldLog = true
			loggedSamplingRate = nil
			shadowLogged = nil
		}
	}()

	shadowShouldLog := true
	env := l.options.GetSDKEnvironmentTier()
	samplingMode, _ = l.SDKConfigs.GetConfigStrValue("sampling_mode")
	specialCaseSamplingRate, _ := l.SDKConfigs.GetConfigIntValue("special_case_sampling_rate")
	specialCaseRules := map[string]bool{
		"disabled": true,
		"default":  true,
		"":         true,
	}

	if strings.HasSuffix(result.RuleID, ":override") || strings.HasSuffix(result.RuleID, ":id_override") {
		return true, nil, nil, samplingMode
	}

	if samplingMode == "" || samplingMode == "none" || env != "production" {
		return true, nil, nil, samplingMode
	}

	if result.ForwardAllExposures {
		return true, nil, nil, samplingMode
	}

	if result.HasSeenAnalyticalGates {
		return true, nil, nil, samplingMode
	}

	samplingSetKey := fmt.Sprintf("%s_%s", name, result.RuleID)
	if !l.samplingKeySet.Contains(samplingSetKey) {
		l.samplingKeySet.Add(samplingSetKey)
		return true, nil, nil, samplingMode
	}

	shouldSample := result.SamplingRate != nil || specialCaseRules[result.RuleID]
	if !shouldSample {
		return true, nil, nil, samplingMode
	}

	var exposureKey string
	switch entityType {
	case EntityGate:
		exposureKey = computeDedupeKeyForGate(name, result.RuleID, result.Value,
			user.UserID, user.CustomIDs)
	case EntityConfig:
		exposureKey = computeDedupeKeyForConfig(name, result.RuleID, user.UserID, user.CustomIDs)
	case EntityLayer:
		exposureKey = computeDedupeKeyForLayer(name, allocatedExperiment, paramName,
			result.RuleID, user.UserID, user.CustomIDs)
	}

	if result.SamplingRate != nil {
		shadowShouldLog = isHashInSamplingRate(exposureKey, *result.SamplingRate)
		loggedSamplingRate = result.SamplingRate
	} else if specialCaseRules[result.RuleID] && specialCaseSamplingRate != 0 {
		shadowShouldLog = isHashInSamplingRate(exposureKey, specialCaseSamplingRate)
		loggedSamplingRate = &specialCaseSamplingRate
	}

	var shadowLoggedStr *string
	if loggedSamplingRate != nil {
		if shadowShouldLog {
			logged := "logged"
			shadowLoggedStr = &logged
		} else {
			dropped := "dropped"
			shadowLoggedStr = &dropped
		}
	}

	switch samplingMode {
	case "on":
		return shadowShouldLog, loggedSamplingRate, shadowLoggedStr, samplingMode
	case "shadow":
		return true, loggedSamplingRate, shadowLoggedStr, samplingMode
	default:
		return true, nil, nil, samplingMode
	}
}

func bigQueryHash(s string) int64 {
	h := sha256.New()
	h.Write([]byte(s))
	sum := h.Sum(nil)

	num := binary.BigEndian.Uint64(sum[:8])

	return int64(num)
}

func isHashInSamplingRate(key string, samplingRate int) bool {
	return bigQueryHash(key)%int64(samplingRate) == 0
}

func computeUserKey(userID string, customIDs map[string]string) string {
	userKey := "u:" + userID + ";"

	if len(customIDs) > 0 {
		if len(customIDs) > 0 {
			for k, v := range customIDs {
				userKey += k + ":" + v + ";"
			}
		}
	}

	return userKey
}

func computeDedupeKeyForGate(gateName, ruleID string, value bool, userID string, customIDs map[string]string) string {
	return "n:" + gateName + ";u:" + computeUserKey(userID, customIDs) + "r:" + ruleID + ";v:" + strconv.FormatBool(value)
}

func computeDedupeKeyForConfig(configName, ruleID, userID string, customIDs map[string]string) string {
	return "n:" + configName + ";u:" + computeUserKey(userID, customIDs) + "r:" + ruleID
}

func computeDedupeKeyForLayer(layerName, experimentName, parameterName, ruleID, userID string, customIDs map[string]string) string {
	return "n:" + layerName + ";e:" + experimentName + ";p:" + parameterName + ";u:" + computeUserKey(userID, customIDs) + "r:" + ruleID
}
