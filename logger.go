package statsig

import (
	"fmt"
	"strconv"
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
	EventName          ExposureEventName   `json:"eventName"`
	User               User                `json:"user"`
	Value              string              `json:"value"`
	Metadata           map[string]string   `json:"metadata"`
	SecondaryExposures []SecondaryExposure `json:"secondaryExposures"`
	Time               int64               `json:"time"`
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
	events        []interface{}
	transport     *transport
	tick          *time.Ticker
	mu            sync.Mutex
	maxEvents     int
	disabled      bool
	diagnostics   *diagnostics
	options       *Options
	errorBoundary *errorBoundary
}

func newLogger(transport *transport, options *Options, diagnostics *diagnostics, errorBoundary *errorBoundary) *logger {
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
		events:        make([]interface{}, 0),
		transport:     transport,
		tick:          time.NewTicker(loggingInterval),
		maxEvents:     maxEvents,
		disabled:      disabled,
		diagnostics:   diagnostics,
		options:       options,
		errorBoundary: errorBoundary,
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
	evt := l.getGateExposureWithEvaluationDetails(user, gateName, res, context)
	l.logExposure(*evt)
	return evt
}

func (l *logger) getGateExposureWithEvaluationDetails(
	user User,
	gateName string,
	res *evalResult,
	context *evalContext,
) *ExposureEvent {
	metadata := map[string]string{
		"gate":      gateName,
		"gateValue": strconv.FormatBool(res.Value),
		"ruleID":    res.RuleID,
	}
	if context != nil && context.IsManualExposure {
		metadata["isManualExposure"] = "true"
	}

	evt := &ExposureEvent{
		User:               user,
		EventName:          GateExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: res.SecondaryExposures,
	}
	l.addEvaluationDetailsToExposureEvent(evt, res.EvaluationDetails)
	l.addDeviceMetadataToExposureEvent(evt, res.DerivedDeviceMetadata)
	return evt
}

func (l *logger) addEvaluationDetailsToExposureEvent(
	evt *ExposureEvent,
	evalDetails *EvaluationDetails,
) {
	if evalDetails != nil {
		evt.Metadata["reason"] = string(evalDetails.detailedReason())
		evt.Metadata["configSyncTime"] = fmt.Sprint(evalDetails.ConfigSyncTime)
		evt.Metadata["initTime"] = fmt.Sprint(evalDetails.InitTime)
		evt.Metadata["serverTime"] = fmt.Sprint(evalDetails.ServerTime)
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

func (l *logger) logConfigExposure(
	user User,
	configName string,
	res *evalResult,
	context *evalContext,
) *ExposureEvent {
	evt := l.getConfigExposureWithEvaluationDetails(user, configName, res, context)
	l.logExposure(*evt)
	return evt
}

func (l *logger) getConfigExposureWithEvaluationDetails(
	user User,
	configName string,
	res *evalResult,
	context *evalContext,
) *ExposureEvent {
	metadata := map[string]string{
		"config":     configName,
		"ruleID":     res.RuleID,
		"rulePassed": strconv.FormatBool(res.Value),
	}
	if context != nil && context.IsManualExposure {
		metadata["isManualExposure"] = "true"
	}
	evt := &ExposureEvent{
		User:               user,
		EventName:          ConfigExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: res.SecondaryExposures,
	}
	l.addEvaluationDetailsToExposureEvent(evt, res.EvaluationDetails)
	l.addDeviceMetadataToExposureEvent(evt, res.DerivedDeviceMetadata)
	return evt
}

func (l *logger) logLayerExposure(
	user User,
	config Layer,
	parameterName string,
	evalResult *evalResult,
	context *evalContext,
) *ExposureEvent {
	evt := l.getLayerExposureWithEvaluationDetails(user, config, parameterName, evalResult, context)
	l.logExposure(*evt)
	return evt
}

func (l *logger) getLayerExposureWithEvaluationDetails(
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
	if context != nil && context.IsManualExposure {
		metadata["isManualExposure"] = "true"
	}

	evt := &ExposureEvent{
		User:               user,
		EventName:          LayerExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.addEvaluationDetailsToExposureEvent(evt, evalResult.EvaluationDetails)
	l.addDeviceMetadataToExposureEvent(evt, evalResult.DerivedDeviceMetadata)
	return evt
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
