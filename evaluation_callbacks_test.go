package statsig

import (
	"testing"
)

func TestExposureCallback(t *testing.T) {
	gateExposures := make(map[string]ExposureEvent)
	configExposures := make(map[string]ExposureEvent)
	experimentExposures := make(map[string]ExposureEvent)
	layerExposures := make(map[string]map[string]ExposureEvent)
	exposures := make(map[string]ExposureEvent)

	evaluationCallbacks := EvaluationCallbacks{
		GateEvaluationCallback: func(name string, result bool, exposure *ExposureEvent) {
			if exposure != nil {
				gateExposures[name] = *exposure
			}
		},
		ConfigEvaluationCallback: func(name string, result DynamicConfig, exposure *ExposureEvent) {
			if exposure != nil {
				configExposures[name] = *exposure
			}
		},
		ExperimentEvaluationCallback: func(name string, result DynamicConfig, exposure *ExposureEvent) {
			if exposure != nil {
				experimentExposures[name] = *exposure
			}
		},
		LayerEvaluationCallback: func(name, param string, result DynamicConfig, exposure *ExposureEvent) {
			if exposure != nil {
				if layerExposures[name] == nil {
					layerExposures[name] = map[string]ExposureEvent{}
				}
				layerExposures[name][param] = *exposure
			}
		},
		ExposureCallback: func(name string, exposure *ExposureEvent) {
			if exposure != nil {
				exposures[name] = *exposure
			}
		},
	}

	testServer := getTestServer(testServerOptions{
		dcsOnline: true,
	})

	var opt *Options
	var user User
	reset := func(includeDisabledExposure bool) {
		opt = &Options{
			API:                  testServer.URL,
			Environment:          Environment{Tier: "test"},
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
			EvaluationCallbacks:  evaluationCallbacks,
		}
		user = User{UserID: "a-user", Email: "a-user@statsig.com"}
		gateExposures = make(map[string]ExposureEvent)
		configExposures = make(map[string]ExposureEvent)
		experimentExposures = make(map[string]ExposureEvent)
		layerExposures = make(map[string]map[string]ExposureEvent)
		exposures = make(map[string]ExposureEvent)
	}

	start := func(includeDisabledExposure bool) {
		reset(includeDisabledExposure)
		opt.EvaluationCallbacks.IncludeDisabledExposures = includeDisabledExposure
		InitializeWithOptions("secret-key", opt)
	}

	t.Run("calls correct exposure callback for all API", func(t *testing.T) {
		start(false)
		CheckGate(user, "always_on_gate")
		GetConfig(user, "test_config")
		GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "a_layer")
		layer.GetBool("layer_param", false)
		ShutdownAndDangerouslyClearInstance()

		if len(gateExposures) != 1 {
			t.Errorf("Should receive exactly 1 gate exposure")
		}

		if len(configExposures) != 1 {
			t.Errorf("Should receive exactly 1 config exposure")
		}

		if len(experimentExposures) != 1 {
			t.Errorf("Should receive exactly 1 experiment exposure")
		}

		if len(layerExposures) != 1 {
			t.Errorf("Should receive exactly 1 layer exposure")
		}

		if len(exposures) != 4 {
			t.Errorf("Should receive exactly 4 exposures")
		}
	})

	t.Run("callback exposure when includeDisabledExposure is on for logging disabled APIs", func(t *testing.T) {
		start(true)
		CheckGateWithExposureLoggingDisabled(user, "always_on_gate")
		GetConfigWithExposureLoggingDisabled(user, "test_config")
		GetExperimentWithExposureLoggingDisabled(user, "sample_experiment")
		layer := GetLayerWithExposureLoggingDisabled(user, "a_layer")
		layer.GetBool("layer_param", false)
		ShutdownAndDangerouslyClearInstance()

		if len(gateExposures) != 1 {
			t.Errorf("Should receive exactly 1 gate exposure")
		}

		if len(configExposures) != 1 {
			t.Errorf("Should receive exactly 1 config exposure")
		}

		if len(experimentExposures) != 1 {
			t.Errorf("Should receive exactly 1 experiment exposure")
		}

		if len(layerExposures) != 1 {
			t.Errorf("Should receive exactly 1 layer exposure")
		}

		if len(exposures) != 4 {
			t.Errorf("Should receive exactly 4 exposures")
		}
	})

	t.Run("no exposure when includeDisabledExposure is off for logging disabled APIs", func(t *testing.T) {
		start(false)
		CheckGateWithExposureLoggingDisabled(user, "always_on_gate")
		GetConfigWithExposureLoggingDisabled(user, "test_config")
		GetExperimentWithExposureLoggingDisabled(user, "sample_experiment")
		layer := GetLayerWithExposureLoggingDisabled(user, "a_layer")
		layer.GetBool("layer_param", false)
		ShutdownAndDangerouslyClearInstance()

		if len(gateExposures) != 0 {
			t.Errorf("Should receive exactly 0 gate exposure")
		}

		if len(configExposures) != 0 {
			t.Errorf("Should receive exactly 0 config exposure")
		}

		if len(experimentExposures) != 0 {
			t.Errorf("Should receive exactly 0 experiment exposure")
		}

		if len(layerExposures) != 0 {
			t.Errorf("Should receive exactly 0 layer exposure")
		}

		if len(exposures) != 0 {
			t.Errorf("Should receive exactly 0 exposures")
		}
	})

	t.Run("exposure callback have holdouts and targeting gate", func(t *testing.T) {
		start(false)
		GetExperiment(user, "experiment_with_holdout_and_gate")
		ShutdownAndDangerouslyClearInstance()

		if len(gateExposures) != 0 {
			t.Errorf("Should receive exactly 0 gate exposure")
		}

		if len(experimentExposures) != 1 {
			t.Errorf("Should receive exactly 1 experiment exposure")
		}

		if len(exposures) != 1 {
			t.Errorf("Should receive exactly 1 exposure")
		}

		secondaryExposure := exposures["experiment_with_holdout_and_gate"].SecondaryExposures
		if len(secondaryExposure) != 2 {
			t.Errorf("Should receive exactly 2 secondary exposures")
		}
		holdout := false
		gate := false
		for _, m := range secondaryExposure {
			if m.Gate == "holdout" {
				holdout = true
			}
			if m.Gate == "employee" {
				gate = true
			}
		}
		if !holdout {
			t.Errorf("Should have holdout exposure")
		}
		if !gate {
			t.Errorf("Should have employee gate exposure")
		}
	})

	t.Run("Doesn't receive exposures in exposure callback when IncludeDisabledExposures is false", func(t *testing.T) {
		start(false)
		CheckGateWithExposureLoggingDisabled(user, "always_on_gate")
		GetConfigWithExposureLoggingDisabled(user, "test_config")
		GetExperimentWithExposureLoggingDisabled(user, "sample_experiment")
		layer := GetLayerWithExposureLoggingDisabled(user, "a_layer")
		layer.GetBool("layer_param", false)
		ShutdownAndDangerouslyClearInstance()

		if len(gateExposures) != 0 {
			t.Errorf("Should receive exactly 0 gate exposure")
		}

		if len(configExposures) != 0 {
			t.Errorf("Should receive exactly 0 config exposure")
		}

		if len(experimentExposures) != 0 {
			t.Errorf("Should receive exactly 0 experiment exposure")
		}

		if len(layerExposures) != 0 {
			t.Errorf("Should receive exactly 0 layer exposure")
		}

		if len(exposures) != 0 {
			t.Errorf("Should receive exactly 0 exposure")
		}
	})
}
