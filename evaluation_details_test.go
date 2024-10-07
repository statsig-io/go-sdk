package statsig

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const configSyncTime = 1631638014811

func TestEvaluationDetails(t *testing.T) {
	gateExposures := make(map[string]ExposureEvent)
	configExposures := make(map[string]ExposureEvent)
	experimentExposures := make(map[string]ExposureEvent)
	layerExposures := make(map[string]map[string]ExposureEvent)

	getTestServer := func(dcsOnline bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "download_config_specs") {
				if !dcsOnline {
					res.WriteHeader(http.StatusInternalServerError)
				} else {
					bytes, _ := os.ReadFile("download_config_specs.json")
					res.WriteHeader(http.StatusOK)
					_, _ = res.Write(bytes)
				}
			}
		}))
	}

	evaluationCallbacks := EvaluationCallbacks{
		GateEvaluationCallback: func(name string, result bool, exposure *ExposureEvent) {
			gateExposures[name] = *exposure
		},
		ConfigEvaluationCallback: func(name string, result DynamicConfig, exposure *ExposureEvent) {
			configExposures[name] = *exposure
		},
		ExperimentEvaluationCallback: func(name string, result DynamicConfig, exposure *ExposureEvent) {
			experimentExposures[name] = *exposure
		},
		LayerEvaluationCallback: func(name, param string, result DynamicConfig, exposure *ExposureEvent) {
			if layerExposures[name] == nil {
				layerExposures[name] = map[string]ExposureEvent{}
			}
			layerExposures[name][param] = *exposure
		},
	}

	var opt *Options
	var user User
	reset := func() {
		opt = &Options{
			API:                  getTestServer(true).URL,
			Environment:          Environment{Tier: "test"},
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
			EvaluationCallbacks:  evaluationCallbacks,
		}
		user = User{UserID: "some_user_id"}
		gateExposures = make(map[string]ExposureEvent)
		configExposures = make(map[string]ExposureEvent)
		experimentExposures = make(map[string]ExposureEvent)
		layerExposures = make(map[string]map[string]ExposureEvent)
	}

	start := func() {
		reset()
		InitializeWithOptions("secret-key", opt)
	}

	startDCSOffline := func() {
		reset()
		opt.API = getTestServer(false).URL
		InitializeWithOptions("secret-key", opt)
	}

	startWithBootstrap := func() {
		reset()
		bytes, _ := os.ReadFile("download_config_specs.json")
		opt.BootstrapValues = string(bytes)
		opt.API = getTestServer(false).URL
		InitializeWithOptions("secret-key", opt)
	}

	t.Run("network init reason", func(t *testing.T) {
		start()
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		_ = GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "a_layer")
		layer.GetBool("layer_param", false)
		ShutdownAndDangerouslyClearInstance()

		numEvents := len(gateExposures) + len(configExposures) + len(experimentExposures) + len(layerExposures)
		if numEvents != 4 {
			t.Errorf("Should receive exactly 4 log_event. Got %d", numEvents)
		}

		compareMetadata(t, gateExposures["always_on_gate"].Metadata, map[string]string{
			"gate":      "always_on_gate",
			"gateValue": "true",
			"ruleID":    "6N6Z8ODekNYZ7F8gFdoLP5",
			"reason":    "Network",
		}, configSyncTime)

		compareMetadata(t, configExposures["test_config"].Metadata, map[string]string{
			"config": "test_config",
			"ruleID": "default",
			"reason": "Network",
		}, configSyncTime)

		compareMetadata(t, experimentExposures["sample_experiment"].Metadata, map[string]string{
			"config": "sample_experiment",
			"ruleID": "2RamGsERWbWMIMnSfOlQuX",
			"reason": "Network",
		}, configSyncTime)

		compareMetadata(t, layerExposures["a_layer"]["layer_param"].Metadata, map[string]string{
			"config":        "a_layer",
			"ruleID":        "2RamGsERWbWMIMnSfOlQuX",
			"parameterName": "layer_param",
			"reason":        "Network",
		}, configSyncTime)
	})

	t.Run("bootstrap init reason", func(t *testing.T) {
		startWithBootstrap()
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		_ = GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		ShutdownAndDangerouslyClearInstance()

		numEvents := len(gateExposures) + len(configExposures) + len(experimentExposures)
		if numEvents != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", numEvents)
		}

		compareMetadata(t, gateExposures["always_on_gate"].Metadata, map[string]string{
			"gate":      "always_on_gate",
			"gateValue": "true",
			"ruleID":    "6N6Z8ODekNYZ7F8gFdoLP5",
			"reason":    "Bootstrap",
		}, configSyncTime)

		compareMetadata(t, configExposures["test_config"].Metadata, map[string]string{
			"config": "test_config",
			"ruleID": "default",
			"reason": "Bootstrap",
		}, configSyncTime)

		compareMetadata(t, experimentExposures["sample_experiment"].Metadata, map[string]string{
			"config": "sample_experiment",
			"ruleID": "2RamGsERWbWMIMnSfOlQuX",
			"reason": "Bootstrap",
		}, configSyncTime)
	})

	t.Run("unrecognized eval reason", func(t *testing.T) {
		startDCSOffline()
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		_ = GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "unallocated_layer")
		layer.GetNumber("an_int", 0)
		ShutdownAndDangerouslyClearInstance()

		numEvents := len(gateExposures) + len(configExposures) + len(experimentExposures)
		if numEvents != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", numEvents)
		}

		compareMetadata(t, gateExposures["always_on_gate"].Metadata, map[string]string{
			"gate":      "always_on_gate",
			"gateValue": "false",
			"ruleID":    "",
			"reason":    "Uninitialized:Unrecognized",
		}, 0)

		compareMetadata(t, configExposures["test_config"].Metadata, map[string]string{
			"config": "test_config",
			"ruleID": "",
			"reason": "Uninitialized:Unrecognized",
		}, 0)

		compareMetadata(t, experimentExposures["sample_experiment"].Metadata, map[string]string{
			"config": "sample_experiment",
			"ruleID": "",
			"reason": "Uninitialized:Unrecognized",
		}, 0)
	})

	t.Run("local override eval reason", func(t *testing.T) {
		start()
		OverrideGate("always_on_gate", false)
		OverrideConfig("test_config", map[string]interface{}{})
		_ = CheckGate(user, "always_on_gate")
		_ = GetConfig(user, "test_config")
		ShutdownAndDangerouslyClearInstance()

		numEvents := len(gateExposures) + len(configExposures) + len(experimentExposures)
		if numEvents != 2 {
			t.Errorf("Should receive exactly 2 log_event. Got %d", numEvents)
		}

		compareMetadata(t, gateExposures["always_on_gate"].Metadata, map[string]string{
			"gate":      "always_on_gate",
			"gateValue": "false",
			"ruleID":    "override",
			"reason":    "Network:LocalOverride",
		}, configSyncTime)

		compareMetadata(t, configExposures["test_config"].Metadata, map[string]string{
			"config": "test_config",
			"ruleID": "override",
			"reason": "Network:LocalOverride",
		}, configSyncTime)
	})
}
