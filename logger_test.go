package statsig

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {}))
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	transport := newTransport("secret", opt)
	errorBoundary := newErrorBoundary("secret", opt, nil)
	sdkConfigs := newSDKConfigs()
	logger := newLogger(transport, opt, nil, errorBoundary, sdkConfigs)

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
	exposures := []SecondaryExposure{{Gate: "another_gate", GateValue: "true", RuleID: "default"}}
	gateRes := &evalResult{RuleID: "rule_id", SecondaryExposures: exposures, Value: true}
	logger.logGateExposure(user, "test_gate", gateRes, nil)
	evt2, ok := logger.events[1].(ExposureEvent)
	if !ok {
		t.Errorf("Gate exposure event type incorrect.")
	}

	gateExposureEvent := ExposureEvent{EventName: GateExposureEventName, User: privateUser, Metadata: map[string]string{
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
	exposures = append(exposures, SecondaryExposure{Gate: "yet_another_gate", GateValue: "false", RuleID: ""})
	configRes := &evalResult{RuleID: "rule_id_config", SecondaryExposures: exposures}
	logger.logConfigExposure(user, "test_config", configRes, nil)
	evt3, ok := logger.events[2].(ExposureEvent)
	if !ok {
		t.Errorf("Config exposure event type incorrect.")
	}

	configExposureEvent := ExposureEvent{EventName: ConfigExposureEventName, User: privateUser, Metadata: map[string]string{
		"config":     "test_config",
		"ruleID":     "rule_id_config",
		"rulePassed": "false",
	}, SecondaryExposures: exposures, Time: evt3.Time}

	if !reflect.DeepEqual(evt3, configExposureEvent) {
		t.Errorf("Config exposure not logged correctly.")
	}
	if evt3.Time/1000 < nowSecond-2 || evt3.Time/1000 > nowSecond+2 {
		t.Errorf("Config exposure event time not set correctly.")
	}
}

func TestLogger_DedupeTTLSetMaxSizeOption(t *testing.T) {
	maxSize := minTTLSetMaxSize + 1
	logger := newLogger(nil, &Options{
		StatsigLoggerOptions: StatsigLoggerOptions{
			DedupeSetMaxSize: maxSize,
		},
	}, nil, nil, newSDKConfigs())

	if logger.dedupeKeySet.maxSize != maxSize {
		t.Fatalf("expected dedupe set max size to be %d, got %d", maxSize, logger.dedupeKeySet.maxSize)
	}

	if logger.shouldDedupeExposure("key1") {
		t.Errorf("first key should not be deduped")
	}

	if logger.shouldDedupeExposure("key2") {
		t.Errorf("second key should not be deduped immediately")
	}

	if !logger.shouldDedupeExposure("key1") {
		t.Errorf("key1 should be deduped")
	}

	for i := 0; i < minTTLSetMaxSize; i++ {
		logger.shouldDedupeExposure(fmt.Sprintf("filler-key-%d", i))
	}

	if logger.shouldDedupeExposure("key1") {
		t.Errorf("key1 should have been cleared after max size reached")
	}
}

func TestLogger_SamplingTTLSetMaxSizeOption(t *testing.T) {
	logger := newLogger(nil, &Options{
		StatsigLoggerOptions: StatsigLoggerOptions{
			SamplingSetMaxSize: minTTLSetMaxSize + 2,
		},
	}, nil, nil, newSDKConfigs())

	if logger.samplingKeySet.maxSize != minTTLSetMaxSize+2 {
		t.Fatalf("expected sampling set max size to be %d, got %d", minTTLSetMaxSize+2, logger.samplingKeySet.maxSize)
	}

	logger.samplingKeySet.Add("sample1")
	logger.samplingKeySet.Add("sample2")
	if !logger.samplingKeySet.Contains("sample1") {
		t.Errorf("sample1 should be tracked")
	}
	if !logger.samplingKeySet.Contains("sample2") {
		t.Errorf("sample2 should be tracked")
	}
	if !logger.samplingKeySet.Contains("sample1") || !logger.samplingKeySet.Contains("sample2") {
		t.Errorf("sampling set should include both sample keys")
	}
}

func TestLogger_DedupeAndSamplingTTLSetMaxSizesAreIndependent(t *testing.T) {
	logger := newLogger(nil, &Options{
		StatsigLoggerOptions: StatsigLoggerOptions{
			DedupeSetMaxSize:   minTTLSetMaxSize + 1,
			SamplingSetMaxSize: minTTLSetMaxSize + 2,
		},
	}, nil, nil, newSDKConfigs())

	if logger.dedupeKeySet.maxSize != minTTLSetMaxSize+1 {
		t.Fatalf("expected dedupe set max size to be %d, got %d", minTTLSetMaxSize+1, logger.dedupeKeySet.maxSize)
	}
	if logger.samplingKeySet.maxSize != minTTLSetMaxSize+2 {
		t.Fatalf("expected sampling set max size to be %d, got %d", minTTLSetMaxSize+2, logger.samplingKeySet.maxSize)
	}
}
