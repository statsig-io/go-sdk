package statsig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEvaluationDetails(t *testing.T) {
	events := []Event{}

	getTestServer := func(dcsOnline bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "download_config_specs") {
				if !dcsOnline {
					res.WriteHeader(http.StatusInternalServerError)
				} else {
					var in *downloadConfigsInput
					bytes, _ := os.ReadFile("download_config_specs.json")
					_ = json.NewDecoder(req.Body).Decode(&in)
					res.WriteHeader(http.StatusOK)
					_, _ = res.Write(bytes)
				}
			} else if strings.Contains(req.URL.Path, "log_event") {
				type requestInput struct {
					Events          []Event         `json:"events"`
					StatsigMetadata statsigMetadata `json:"statsigMetadata"`
				}
				input := &requestInput{}
				defer req.Body.Close()
				buf := new(bytes.Buffer)
				_, _ = buf.ReadFrom(req.Body)

				_ = json.Unmarshal(buf.Bytes(), &input)
				events = input.Events
			}
		}))
	}

	var opt *Options
	var user User
	reset := func() {
		opt = &Options{
			API:         getTestServer(true).URL,
			Environment: Environment{Tier: "test"},
		}
		user = User{UserID: "some_user_id"}
		events = []Event{}
	}

	var mockedServerTime int64
	doMock := func() {
		now = func() time.Time { return time.Date(2022, 12, 12, 0, 0, 0, 0, time.Local) }
		mockedServerTime = now().Unix() * 1000
	}

	start := func() {
		reset()
		doMock()
		InitializeWithOptions("secret-key", opt)
	}

	startDCSOffline := func() {
		reset()
		opt.API = getTestServer(false).URL
		doMock()
		InitializeWithOptions("secret-key", opt)
	}

	startWithBootstrap := func() {
		reset()
		bytes, _ := os.ReadFile("download_config_specs.json")
		opt.BootstrapValues = string(bytes)
		opt.API = getTestServer(false).URL
		doMock()
		InitializeWithOptions("secret-key", opt)
	}

	t.Run("network init reason", func(t *testing.T) {
		start()
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		_ = GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		shutDownAndClearInstance()

		if len(events) != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", len(events))
		}

		if reflect.DeepEqual(events[0].Metadata, map[string]string{
			"gate":           "always_on_gate",
			"gateValue":      "true",
			"ruleID":         "6N6Z8ODekNYZ7F8gFdoLP5",
			"reason":         "Network",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[0].Metadata)
		}
		if reflect.DeepEqual(events[1].Metadata, map[string]string{
			"config":         "test_config",
			"ruleID":         "default",
			"reason":         "Network",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[1].Metadata)
		}
		if reflect.DeepEqual(events[2].Metadata, map[string]string{
			"config":         "sample_experiment",
			"ruleID":         "2RamGsERWbWMIMnSfOlQuX",
			"reason":         "Network",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[2].Metadata)
		}
	})

	t.Run("bootstrap init reason", func(t *testing.T) {
		startWithBootstrap()
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		_ = GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		shutDownAndClearInstance()

		if len(events) != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", len(events))
		}

		if reflect.DeepEqual(events[0].Metadata, map[string]string{
			"gate":           "always_on_gate",
			"gateValue":      "true",
			"ruleID":         "6N6Z8ODekNYZ7F8gFdoLP5",
			"reason":         "Bootstrap",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[0].Metadata)
		}
		if reflect.DeepEqual(events[1].Metadata, map[string]string{
			"config":         "test_config",
			"ruleID":         "default",
			"reason":         "Bootstrap",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[1].Metadata)
		}
		if reflect.DeepEqual(events[2].Metadata, map[string]string{
			"config":         "sample_experiment",
			"ruleID":         "2RamGsERWbWMIMnSfOlQuX",
			"reason":         "Bootstrap",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[2].Metadata)
		}
	})

	t.Run("unrecognized eval reason", func(t *testing.T) {
		startDCSOffline()
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		_ = GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		shutDownAndClearInstance()

		if len(events) != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", len(events))
		}

		if reflect.DeepEqual(events[0].Metadata, map[string]string{
			"gate":           "always_on_gate",
			"gateValue":      "false",
			"ruleID":         "",
			"reason":         "Unrecognized",
			"configSyncTime": "0",
			"initTime":       "0",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[0].Metadata)
		}
		if reflect.DeepEqual(events[1].Metadata, map[string]string{
			"config":         "test_config",
			"ruleID":         "",
			"reason":         "Unrecognized",
			"configSyncTime": "0",
			"initTime":       "0",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[1].Metadata)
		}
		if reflect.DeepEqual(events[2].Metadata, map[string]string{
			"config":         "sample_experiment",
			"ruleID":         "",
			"reason":         "Unrecognized",
			"configSyncTime": "0",
			"initTime":       "0",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[2].Metadata)
		}
	})

	t.Run("local override eval reason", func(t *testing.T) {
		start()
		OverrideGate("always_on_gate", false)
		OverrideConfig("test_config", map[string]interface{}{})
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		shutDownAndClearInstance()

		if len(events) != 2 {
			t.Errorf("Should receive exactly 2 log_event. Got %d", len(events))
		}

		if reflect.DeepEqual(events[0].Metadata, map[string]string{
			"gate":           "always_on_gate",
			"gateValue":      "false",
			"ruleID":         "override",
			"reason":         "LocalOverride",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[0].Metadata)
		}
		if reflect.DeepEqual(events[1].Metadata, map[string]string{
			"config":         "test_config",
			"ruleID":         "override",
			"reason":         "LocalOverride",
			"configSyncTime": "1631638014811",
			"initTime":       "1631638014811",
			"serverTime":     fmt.Sprint(mockedServerTime),
		}) == false {
			t.Errorf("Invalid metadata %v", events[1].Metadata)
		}
	})
}
