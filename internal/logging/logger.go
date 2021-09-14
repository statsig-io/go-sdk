package logging

import (
	"strconv"
	"time"

	"github.com/statsig-io/go-sdk/internal/net"
	"github.com/statsig-io/go-sdk/types"
)

type exposureEvent struct {
	EventName          string              `json:"eventName"`
	User               types.StatsigUser   `json:"user"`
	Value              string              `json:"value"`
	Metadata           map[string]string   `json:"metadata"`
	SecondaryExposures []map[string]string `json:"secondaryExposures"`
}

type logEventInput struct {
	Events          []interface{}       `json:"events"`
	StatsigMetadata net.StatsigMetadata `json:"statsigMetadata"`
}

type logEventResponse struct{}

const MaxEvents = 500
const GateExposureEvent = "statsig::gate_exposure"
const ConfigExposureEvent = "statsig::config_exposure"

type Logger struct {
	events []interface{}
	net    *net.Net
	tick   *time.Ticker
}

func New(net *net.Net) *Logger {
	log := &Logger{
		events: make([]interface{}, 0),
		net:    net,
		tick:   time.NewTicker(time.Minute),
	}

	go log.backgroundFlush()

	return log
}

func (l *Logger) backgroundFlush() {
	for range l.tick.C {
		l.Flush(false)
	}
}

func (l *Logger) Log(evt interface{}) {
	switch evt.(type) {
	case exposureEvent:
		event := evt.(exposureEvent)
		event.User.PrivateAttributes = nil
		l.events = append(l.events, event)
	case types.StatsigEvent:
		event := evt.(types.StatsigEvent)
		event.User.PrivateAttributes = nil
		l.events = append(l.events, event)
	default:
		return
	}

	if len(l.events) >= MaxEvents {
		l.Flush(false)
	}
}

func (l *Logger) LogGateExposure(
	user types.StatsigUser,
	gateName string,
	value bool,
	ruleID string,
	exposures []map[string]string,
) {
	evt := &exposureEvent{
		User:      user,
		EventName: GateExposureEvent,
		Metadata: map[string]string{
			"gate":      gateName,
			"gateValue": strconv.FormatBool(value),
			"ruleID":    ruleID,
		},
		SecondaryExposures: exposures,
	}
	l.Log(*evt)
}

func (l *Logger) LogConfigExposure(
	user types.StatsigUser,
	configName string,
	ruleID string,
	exposures []map[string]string,
) {
	evt := &exposureEvent{
		User:      user,
		EventName: ConfigExposureEvent,
		Metadata: map[string]string{
			"config": configName,
			"ruleID": ruleID,
		},
		SecondaryExposures: exposures,
	}
	l.Log(*evt)
}

func (l *Logger) Flush(closing bool) {
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

func (l *Logger) sendEvents(events []interface{}) {
	input := &logEventInput{
		Events:          events,
		StatsigMetadata: l.net.GetStatsigMetadata(),
	}
	var res logEventResponse
	l.net.RetryablePostRequest("/log_event", input, &res, net.MaxRetries)
}
