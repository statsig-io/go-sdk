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
	SecondaryExposures []map[string]string `json:"secondaryExposures"`
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

type logContext struct {
	isManualExposure bool
}

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

func (l *logger) logExposureWithEvaluationDetails(
	evt *ExposureEvent,
	evalDetails *EvaluationDetails,
) {
	if evalDetails != nil {
		evt.Metadata["reason"] = string(evalDetails.Reason)
		evt.Metadata["configSyncTime"] = fmt.Sprint(evalDetails.ConfigSyncTime)
		evt.Metadata["initTime"] = fmt.Sprint(evalDetails.InitTime)
		evt.Metadata["serverTime"] = fmt.Sprint(evalDetails.ServerTime)
	}
	l.logExposure(*evt)

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
	value bool,
	ruleID string,
	exposures []map[string]string,
	evalDetails *EvaluationDetails,
	context *logContext,
) *ExposureEvent {
	metadata := map[string]string{
		"gate":      gateName,
		"gateValue": strconv.FormatBool(value),
		"ruleID":    ruleID,
	}
	if context != nil && context.isManualExposure {
		metadata["isManualExposure"] = "true"
	}
	evt := &ExposureEvent{
		User:               user,
		EventName:          GateExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.logExposureWithEvaluationDetails(evt, evalDetails)
	return evt
}

func (l *logger) logConfigExposure(
	user User,
	configName string,
	ruleID string,
	exposures []map[string]string,
	evalDetails *EvaluationDetails,
	context *logContext,
) *ExposureEvent {
	metadata := map[string]string{
		"config": configName,
		"ruleID": ruleID,
	}
	if context != nil && context.isManualExposure {
		metadata["isManualExposure"] = "true"
	}
	evt := &ExposureEvent{
		User:               user,
		EventName:          ConfigExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.logExposureWithEvaluationDetails(evt, evalDetails)
	return evt
}

func (l *logger) logLayerExposure(
	user User,
	config Layer,
	parameterName string,
	evalResult evalResult,
	evalDetails *EvaluationDetails,
	context *logContext,
) *ExposureEvent {
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

	evt := &ExposureEvent{
		User:               user,
		EventName:          LayerExposureEventName,
		Metadata:           metadata,
		SecondaryExposures: exposures,
	}
	l.logExposureWithEvaluationDetails(evt, evalDetails)
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
		message := fmt.Sprintf("Failed to log %d events afrer %d retries, dropping the request", len(events), maxRetries)
		extra := map[string]interface{}{
			"eventCount": len(events),
		}
		options := logExceptionOptions{
			Tag:          "statsig::log_event_failed",
			Extra:        &extra,
			BypassDedupe: true,
		}
		l.errorBoundary.logExceptionWithOptions(toError(message), options)
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
