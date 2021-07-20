package statsig

import (
	"sync"
	"statsig/internal/net"
	"statsig/pkg/types"
)

const MaxEvents = 500

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

func (l *statsigLogger) flush() {
	l.net.LogEvents(l.events)
	l.events = make([]types.StatsigEvent, 0)
}
