package logging

import (
	"strconv"
	"time"

	"github.com/statsig-io/go-sdk/internal/net"
	"github.com/statsig-io/go-sdk/types"
)

type logEventInput struct {
	Events          []types.StatsigEvent `json:"events"`
	StatsigMetadata net.StatsigMetadata  `json:"statsigMetadata"`
}

type logEventResponse struct{}

const MaxEvents = 500
const GateExposureEvent = "statsig::gate_exposure"
const ConfigExposureEvent = "statsig::config_exposure"

type Logger struct {
	events []types.StatsigEvent
	net    *net.Net
	tick   *time.Ticker
}

func New(net *net.Net) *Logger {
	log := &Logger{
		events: make([]types.StatsigEvent, 0),
		net:    net,
		tick:   time.NewTicker(time.Second * time.Duration(5)),
	}

	go log.backgroundFlush()

	return log
}

func (l *Logger) backgroundFlush() {
	for range l.tick.C {
		l.Flush(false)
	}
}

func (l *Logger) Log(evt types.StatsigEvent) {
	l.events = append(l.events, evt)
	if len(l.events) >= MaxEvents {
		l.Flush(false)
	}
}

func (l *Logger) LogGateExposure(
	user types.StatsigUser,
	gateName string,
	value bool,
	ruleID string,
) {
	evt := &types.StatsigEvent{
		User:      user,
		EventName: GateExposureEvent,
		Metadata: map[string]string{
			"gate":      gateName,
			"gateValue": strconv.FormatBool(value),
			"ruleID":    ruleID,
		},
	}
	l.Log(*evt)
}

func (l *Logger) LogConfigExposure(
	user types.StatsigUser,
	configName string,
	ruleID string,
) {
	evt := &types.StatsigEvent{
		User:      user,
		EventName: ConfigExposureEvent,
		Metadata: map[string]string{
			"config": configName,
			"ruleID": ruleID,
		},
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
	l.logEvents(l.events)
	l.events = make([]types.StatsigEvent, 0)
}

func (l *Logger) logEvents(events []types.StatsigEvent) {
	input := &logEventInput{
		Events:          events,
		StatsigMetadata: l.net.GetStatsigMetadata(),
	}
	var res logEventResponse
	l.net.PostRequest("/log_event", input, &res)
}
