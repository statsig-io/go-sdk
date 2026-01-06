package statsig

import (
	"testing"
)

func TestExposureLogging(t *testing.T) {
	events := []Event{}

	testServer := getTestServer(testServerOptions{
		onLogEvent: func(newEvents []map[string]interface{}) {
			for _, newEvent := range newEvents {
				eventTyped := convertToExposureEvent(newEvent)
				events = append(events, eventTyped)
			}
		},
	})

	opt := &Options{
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}

	user := User{UserID: "some_user_id", Email: "someuser@statsig.com"}

	start := func() {
		events = []Event{}
		InitializeWithOptions("secret-key", opt)
	}

	t.Run("logs exposures for all API", func(t *testing.T) {
		start()
		gateValue := CheckGate(user, "always_on_gate")
		gate := GetGate(User{UserID: "some_user_id_again", Email: "someuser@statsig.com"}, "always_on_gate")
		config := GetConfig(user, "test_config")
		experiment := GetExperiment(user, "sample_experiment")
		layer := GetLayer(user, "a_layer")
		layer.GetString("experiment_param", "")
		layerNameExist, existOk := GetExperimentLayer("sample_experiment")
		_, nonExistOk := GetExperimentLayer("non_exist_experiment")
		ShutdownAndDangerouslyClearInstance()

		if len(events) != 5 {
			t.Errorf("Should receive exactly 5 log_events")
		}

		if !existOk {
			t.Errorf("Layer name should exist for experiment sample_experiment")
		}

		if layerNameExist != "a_layer" {
			t.Errorf("Layer name should be a_layer for experiment sample_experiment")
		}

		if nonExistOk {
			t.Errorf("Layer name should not exist for non_exist_experiment")
		}

		if gateValue != gate.Value {
			t.Errorf("CheckGate and GetGate returned different results: %+v vs %+v", gateValue, gate.Value)
		}
		if gate.GroupName != "everyone" {
			t.Errorf("Gate expected group name %+v but received %+v", "everyone", gate.GroupName)
		}
		if config.GroupName != "statsig email" {
			t.Errorf("Config expected group name %+v but received %+v", "statsig email", config.GroupName)
		}
		if experiment.GroupName != "Control" {
			t.Errorf("Experiment expected group name %+v but received %+v", "Control", experiment.GroupName)
		}
		if layer.GroupName != "Control" {
			t.Errorf("Layer expected group name %+v but received %+v", "Control", layer.GroupName)
		}
	})

	//

	t.Run("does not log for exposure logging disabled API", func(t *testing.T) {
		start()
		CheckGateWithExposureLoggingDisabled(user, "always_on_gate")
		GetGateWithExposureLoggingDisabled(user, "always_on_gate")
		GetConfigWithExposureLoggingDisabled(user, "test_config")
		GetExperimentWithExposureLoggingDisabled(user, "sample_experiment")
		layer := GetLayerWithExposureLoggingDisabled(user, "a_layer")
		layer.GetString("experiment_param", "")
		ShutdownAndDangerouslyClearInstance()

		if len(events) != 0 {
			t.Errorf("Should receive no log_event")
		}
	})

	//

	t.Run("logs for manually log exposure API", func(t *testing.T) {
		start()
		ManuallyLogGateExposure(user, "always_on_gate")
		ManuallyLogConfigExposure(user, "test_config")
		ManuallyLogExperimentExposure(user, "sample_experiment")
		ManuallyLogLayerParameterExposure(user, "a_layer", "experiment_param")
		ShutdownAndDangerouslyClearInstance()

		if len(events) != 4 {
			t.Errorf("Should receive exactly 4 log_events")
		}

		gateExposure := events[0]
		if gateExposure.EventName != "statsig::gate_exposure" {
			t.Errorf("Incorrect exposure name")
		}
		if gateExposure.Metadata["gate"] != "always_on_gate" {
			t.Errorf("Incorrect value for gate in metadata")
		}
		if gateExposure.Metadata["isManualExposure"] != "true" {
			t.Errorf("Incorrect value for isManualExposure in metadata")
		}
		configExposure := events[1]
		if configExposure.EventName != "statsig::config_exposure" {
			t.Errorf("Incorrect exposure name")
		}
		if configExposure.Metadata["config"] != "test_config" {
			t.Errorf("Incorrect value for config in metadata")
		}
		if configExposure.Metadata["isManualExposure"] != "true" {
			t.Errorf("Incorrect value for isManualExposure in metadata")
		}
		experimentExposure := events[2]
		if experimentExposure.EventName != "statsig::config_exposure" {
			t.Errorf("Incorrect exposure name")
		}
		if experimentExposure.Metadata["config"] != "sample_experiment" {
			t.Errorf("Incorrect value for config in metadata")
		}
		if experimentExposure.Metadata["isManualExposure"] != "true" {
			t.Errorf("Incorrect value for isManualExposure in metadata")
		}
		layerExposure := events[3]
		if layerExposure.EventName != "statsig::layer_exposure" {
			t.Errorf("Incorrect exposure name")
		}
		if layerExposure.Metadata["config"] != "a_layer" {
			t.Errorf("Incorrect value for config in metadata")
		}
		if layerExposure.Metadata["isManualExposure"] != "true" {
			t.Errorf("Incorrect value for isManualExposure in metadata")
		}
	})

	defer testServer.Close()

}
