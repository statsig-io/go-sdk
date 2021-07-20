package logging

import (
	"statsig/internal/net"
	"statsig/pkg/types"
	"strconv"
	"sync"
)

const MaxEvents = 500
const GateExposureEvent = "statsig::gate_exposure"
const ConfigExposureEvent = "statsig::config_exposure"

type Logger struct {
	events []types.StatsigEvent
	net    *net.Net
	sync.Mutex
}

func New(net *net.Net) *Logger {
	return &Logger{
		events: make([]types.StatsigEvent, 0),
		net:    net,
	}
}

func (l *Logger) Log(evt types.StatsigEvent) {
	l.Lock()
	defer l.Unlock()

	l.events = append(l.events, evt)
	if len(l.events) >= MaxEvents {
		l.Flush()
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

func (l *Logger) Flush() {
	l.net.LogEvents(l.events)
	l.events = make([]types.StatsigEvent, 0)
}
