package statsig

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

func TestLog(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		return
	}))
	defer testServer.Close()

	transport := newTransport("secret", testServer.URL, "", "")
	logger := newLogger(transport)

	user := User{
		UserID:            "123",
		Email:             "123@gmail.com",
		PrivateAttributes: map[string]interface{}{"private": "shh"},
	}
	privateUser := User{
		UserID: "123",
		Email:  "123@gmail.com",
	}

	// Test custom logs
	customEvent := Event{
		EventName: "test_event",
		User:      user, Value: "3"}
	customEventNoPrivate := Event{
		EventName: "test_event",
		User:      privateUser, Value: "3"}
	logger.LogCustom(customEvent)

	if !reflect.DeepEqual(logger.events[0], customEventNoPrivate) {
		t.Errorf("Custom event not logged correctly.")
	}

	// Test gate exposures
	exposures := []map[string]string{{"gate": "another_gate", "gateValue": "true", "ruleID": "default"}}
	logger.LogGateExposure(user, "test_gate", true, "rule_id", exposures)
	gateExposureEvent := exposureEvent{EventName: gateExposureEvent, User: privateUser, Metadata: map[string]string{
		"gate":      "test_gate",
		"gateValue": strconv.FormatBool(true),
		"ruleID":    "rule_id",
	}, SecondaryExposures: exposures}

	if !reflect.DeepEqual(logger.events[1], gateExposureEvent) {
		t.Errorf("Gate exposure not logged correctly.")
	}

	// Test config exposures
	exposures = append(exposures, map[string]string{"gate": "yet_another_gate", "gateValue": "false", "ruleID": ""})
	logger.LogConfigExposure(user, "test_config", "rule_id_config", exposures)
	configExposureEvent := exposureEvent{EventName: configExposureEvent, User: privateUser, Metadata: map[string]string{
		"config": "test_config",
		"ruleID": "rule_id_config",
	}, SecondaryExposures: exposures}

	if !reflect.DeepEqual(logger.events[2], configExposureEvent) {
		t.Errorf("Config exposure not logged correctly.")
	}
}
