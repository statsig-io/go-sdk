package statsig

import (
	"sync"
	"statsig/internal/net"
	"statsig/pkg/types"
	"strconv"
)

const MaxEvents = 500
const GateExposureEvent = "statsig::gate_exposure"
const ConfigExposureEvent = "statsig::config_exposure"

type statsigLogger struct {
	events []types.StatsigEvent
	net *net.Net
	sync.Mutex
}

func NewLogger(net *net.Net) *statsigLogger {
	return &statsigLogger{
		events: make([]types.StatsigEvent, 0),
		net: net,
	}
}

func (l *statsigLogger) Log(evt types.StatsigEvent) {
	l.Lock()
	defer l.Unlock()

	l.events = append(l.events, evt)
	if (len(l.events) >= MaxEvents) {
		l.flush()
	}
}

func (l *statsigLogger) logGateExposure(
	user types.StatsigUser,
	gateName string,
	value bool,
	ruleID string,
) {
	evt := &types.StatsigEvent{
		User: user,
		EventName: GateExposureEvent,
		Metadata: map[string]string{
			"gate": gateName,
			"gateValue": strconv.FormatBool(value),
			"ruleID": ruleID,
		},
	}
	l.Log(*evt)
}

func (l *statsigLogger) logConfigExposure(
	user types.StatsigUser,
	configName string,
	ruleID string,
) {
	evt := &types.StatsigEvent{
		User: user,
		EventName: ConfigExposureEvent,
		Metadata: map[string]string{
			"config": configName,
			"ruleID": ruleID,
		},
	}
	l.Log(*evt)
}

func (l *statsigLogger) flush() {
	l.net.LogEvents(l.events)
	l.events = make([]types.StatsigEvent, 0)
}
