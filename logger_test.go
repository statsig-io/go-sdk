package statsig

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	InitializeGlobalOutputLogger(OutputLoggerOptions{
		LogCallback: func(message string, err error) {
			t.Log(message)
			t.Log(err)
		},
		DisableInitDiagnostics: false,
		DisableSyncDiagnostics: true,
	})
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {}))
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	transport := newTransport("secret", opt)
	logger := newLogger(transport, opt, nil)

	user := User{
		UserID:            "123",
		Email:             "123@gmail.com",
		PrivateAttributes: map[string]interface{}{"private": "shh"},
	}
	privateUser := User{
		UserID: "123",
		Email:  "123@gmail.com",
	}

	nowSecond := time.Now().Unix()
	// Test custom logs
	customEvent := Event{
		EventName: "test_event",
		User:      user, Value: "3"}
	logger.logCustom(customEvent)
	evt1, ok := logger.events[0].(Event)
	if !ok {
		t.Errorf("Custom event type incorrect.")
	}

	customEventNoPrivate := Event{
		EventName: "test_event",
		User:      privateUser, Value: "3",
		Time: evt1.Time,
	}

	if !reflect.DeepEqual(evt1, customEventNoPrivate) {
		t.Errorf("Custom event not logged correctly.")
	}
	if evt1.Time/1000 < nowSecond-2 || evt1.Time/1000 > nowSecond+2 {
		t.Errorf("Custom event time not set correctly.")
	}

	// Test gate exposures
	exposures := []map[string]string{{"gate": "another_gate", "gateValue": "true", "ruleID": "default"}}
	logger.logGateExposure(user, "test_gate", true, "rule_id", exposures, nil, nil)
	evt2, ok := logger.events[1].(exposureEvent)
	if !ok {
		t.Errorf("Gate exposure event type incorrect.")
	}

	gateExposureEvent := exposureEvent{EventName: gateExposureEventName, User: privateUser, Metadata: map[string]string{
		"gate":      "test_gate",
		"gateValue": strconv.FormatBool(true),
		"ruleID":    "rule_id",
	}, SecondaryExposures: exposures, Time: evt2.Time}

	if !reflect.DeepEqual(evt2, gateExposureEvent) {
		t.Errorf("Gate exposure not logged correctly.")
	}
	if evt2.Time/1000 < nowSecond-2 || evt2.Time/1000 > nowSecond+2 {
		t.Errorf("Gate exposure event time not set correctly.")
	}

	// Test config exposures
	exposures = append(exposures, map[string]string{"gate": "yet_another_gate", "gateValue": "false", "ruleID": ""})
	logger.logConfigExposure(user, "test_config", "rule_id_config", exposures, nil, nil)
	evt3, ok := logger.events[2].(exposureEvent)
	if !ok {
		t.Errorf("Config exposure event type incorrect.")
	}

	configExposureEvent := exposureEvent{EventName: configExposureEventName, User: privateUser, Metadata: map[string]string{
		"config": "test_config",
		"ruleID": "rule_id_config",
	}, SecondaryExposures: exposures, Time: evt3.Time}

	if !reflect.DeepEqual(evt3, configExposureEvent) {
		t.Errorf("Config exposure not logged correctly.")
	}
	if evt3.Time/1000 < nowSecond-2 || evt3.Time/1000 > nowSecond+2 {
		t.Errorf("Config exposure event time not set correctly.")
	}

	logger.flush(true)
}
