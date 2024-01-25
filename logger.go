package statsig

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	gateExposureEventName   = "statsig::gate_exposure"
	configExposureEventName = "statsig::config_exposure"
	layerExposureEventName  = "statsig::layer_exposure"
	diagnosticsEventName    = "statsig::diagnostics"
)

type exposureEvent struct {
	EventName          string              `json:"eventName"`
	User               User                `json:"user"`
	Value              string              `json:"value"`
	Metadata           map[string]string   `json:"metadata"`
	SecondaryExposures []map[string]string `json:"secondaryExposures"`
	Time               int64               `json:"time"`
}

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

type logContext struct {
	isManualExposure bool
}

type logger struct {
	events      []interface{}
	transport   *transport
	tick        *time.Ticker
	mu          sync.Mutex
	maxEvents   int
	diagnostics *diagnostics
	options     *Options
}

func newLogger(transport *transport, options *Options, diagnostics *diagnostics) *logger {
	loggingInterval := time.Minute
	maxEvents := 1000
	if options.LoggingInterval > 0 {
		loggingInterval = options.LoggingInterval
	}
	if options.LoggingMaxBufferSize > 0 {
		maxEvents = options.LoggingMaxBufferSize
	}
	log := &logger{
		events:      make([]interface{}, 0),
		transport:   transport,
		tick:        time.NewTicker(loggingInterval),
		maxEvents:   maxEvents,
		diagnostics: diagnostics,
		options:     options,
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

func (l *logger) logExposureWithEvaluationDetails(
	evt *exposureEvent,
	evalDetails *evaluationDetails,
) {
	if evalDetails != nil {
		evt.Metadata["reason"] = string(evalDetails.reason)
		evt.Metadata["configSyncTime"] = fmt.Sprint(evalDetails.configSyncTime)
		evt.Metadata["initTime"] = fmt.Sprint(evalDetails.initTime)
		evt.Metadata["serverTime"] = fmt.Sprint(evalDetails.serverTime)
	}
	l.logExposure(*evt)
}

func (l *logger) logExposure(evt exposureEvent) {
	evt.User.PrivateAttributes = nil
	if evt.Time == 0 {
		evt.Time = getUnixMilli()
	}
	l.logInternal(evt)
}

func (l *logger) logInternal(evt interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, evt)
	if len(l.events) >= l.maxEvents {
		l.flushInternal(false)
	}
}

func (l *logger) logGateExposure(
	user User,
	gateName string,
	value bool,
	ruleID string,
	exposures []map[string]string,
	evalDetails *evaluationDetails,
	context *logContext,
) {
	metadata := map[string]string{
		"gate":      gateName,
		"gateValue": strconv.FormatBool(value),
		"ruleID":    ruleID,
	}
	if context != nil && context.isManualExposure {
		metadata["isManualExposure"] = "true"
	}
	evt := &exposureEvent{
		User:               user,
		EventName:          gateExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.logExposureWithEvaluationDetails(evt, evalDetails)
}

func (l *logger) logConfigExposure(
	user User,
	configName string,
	ruleID string,
	exposures []map[string]string,
	evalDetails *evaluationDetails,
	context *logContext,
) {
	metadata := map[string]string{
		"config": configName,
		"ruleID": ruleID,
	}
	if context != nil && context.isManualExposure {
		metadata["isManualExposure"] = "true"
	}
	evt := &exposureEvent{
		User:               user,
		EventName:          configExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.logExposureWithEvaluationDetails(evt, evalDetails)
}

func (l *logger) logLayerExposure(
	user User,
	config configBase,
	parameterName string,
	evalResult evalResult,
	evalDetails *evaluationDetails,
	context *logContext,
) {
	allocatedExperiment := ""
	exposures := evalResult.UndelegatedSecondaryExposures
	isExplicit := evalResult.ExplicitParameters[parameterName]

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
	if context != nil && context.isManualExposure {
		metadata["isManualExposure"] = "true"
	}

	evt := &exposureEvent{
		User:               user,
		EventName:          layerExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.logExposureWithEvaluationDetails(evt, evalDetails)
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

	eventsCopy := make([]interface{}, len(l.events))
	copy(eventsCopy, l.events)

	if closing {
		l.sendEvents(eventsCopy)
	} else {
		go l.sendEvents(eventsCopy)
	}

	l.events = make([]interface{}, 0)
}

func (l *logger) sendEvents(events []interface{}) {
	input := &logEventInput{
		Events:          events,
		StatsigMetadata: l.transport.metadata,
	}
	var res logEventResponse
	_, err := l.transport.retryablePostRequest("/log_event", input, &res, maxRetries)
	if err != nil {
		global.Logger().LogError(err)
	}
}

func (l *logger) logDiagnosticsEvents(d *diagnostics) {
	l.logDiagnosticsEvent(d.initDiagnostics)
	l.logDiagnosticsEvent(d.syncDiagnostics)
	l.logDiagnosticsEvent(d.apiDiagnostics)
}

func (l *logger) logDiagnosticsEvent(d *diagnosticsBase) {
	if l.options.StatsigLoggerOptions.DisableInitDiagnostics && d.context == InitializeContext {
		return
	}
	if l.options.StatsigLoggerOptions.DisableSyncDiagnostics && d.context == ConfigSyncContext {
		return
	}
	if l.options.StatsigLoggerOptions.DisableApiDiagnostics && d.context == ApiCallContext {
		return
	}
	serialized := d.serializeWithSampling()
	markers, exists := serialized["markers"]
	if !exists {
		return
	}
	markersTyped, ok := markers.([]marker)
	if !ok || len(markersTyped) == 0 {
		return
	}
	event := diagnosticsEvent{
		EventName: diagnosticsEventName,
		Time:      getUnixMilli(),
		Metadata:  serialized,
	}
	d.clearMarkers()
	l.logInternal(event)
}
