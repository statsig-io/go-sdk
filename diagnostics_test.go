package statsig

import (
	"os"
	"sync"
	"testing"
	"time"
)

type Pair struct {
	A string
	B interface{}
}

func TestInitDiagnostics(t *testing.T) {
	var events events
	testServer := getTestServer(testServerOptions{
		dcsOnline: true,
		onLogEvent: func(newEvents []map[string]interface{}) {
			events = newEvents
		},
	})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  true,
		},
	}
	InitializeWithOptions("secret-key", options)
	ShutdownAndDangerouslyClearInstance()

	markers := extractMarkers(events, 0)

	if len(markers) != 14 {
		t.Errorf("Expected %d markers but got %d", 14, len(markers))
	}

	assertMarkerEqual(t, markers[0], "overall", "", "start")
	assertMarkerEqual(t, markers[1], "download_config_specs", "network_request", "start")
	assertMarkerEqual(t, markers[2], "download_config_specs", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[3], "download_config_specs", "process", "start")
	assertMarkerEqual(t, markers[4], "download_config_specs", "process", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[5], "get_id_list_sources", "network_request", "start")
	assertMarkerEqual(t, markers[6], "get_id_list_sources", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[7], "get_id_list_sources", "process", "start", Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[8], "get_id_list", "network_request", "start")
	assertMarkerEqual(t, markers[9], "get_id_list", "network_request", "end", Pair{"statusCode", float64(200)})
	assertMarkerEqual(t, markers[10], "get_id_list", "process", "start")
	assertMarkerEqual(t, markers[11], "get_id_list", "process", "end", Pair{"success", false})
	assertMarkerEqual(t, markers[12], "get_id_list_sources", "process", "end", Pair{"success", true}, Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[13], "overall", "", "end", Pair{"success", true})
}

func TestConfigSyncDiagnostics(t *testing.T) {
	var mu sync.Mutex

	count := 0
	testServer := getTestServer(
		testServerOptions{
			dcsOnline: true,
			onLogEvent: func(events []map[string]interface{}) {
				mu.Lock()
				defer mu.Unlock()
				count += 1

				if count == 1 {
					if len(events) != 1 {
						t.Errorf("Expected 1 diagnostics events, received %d", len(events))
					}

					markers := extractMarkers(events, 0)

					if len(markers) != 12 {
						t.Errorf("Expected %d markers but got %d", 12, len(markers))
					}

					assertMarkerEqual(t, markers[0], "download_config_specs", "network_request", "start")
					assertMarkerEqual(t, markers[1], "download_config_specs", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
					assertMarkerEqual(t, markers[2], "download_config_specs", "process", "start")
					assertMarkerEqual(t, markers[3], "download_config_specs", "process", "end", Pair{"success", true})
					assertMarkerEqual(t, markers[4], "get_id_list_sources", "network_request", "start")
					assertMarkerEqual(t, markers[5], "get_id_list_sources", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
					assertMarkerEqual(t, markers[6], "get_id_list_sources", "process", "start", Pair{"idListCount", float64(1)})
					assertMarkerEqual(t, markers[7], "get_id_list", "network_request", "start")
					assertMarkerEqual(t, markers[8], "get_id_list", "network_request", "end", Pair{"statusCode", float64(200)})
					assertMarkerEqual(t, markers[9], "get_id_list", "process", "start")
					assertMarkerEqual(t, markers[10], "get_id_list", "process", "end", Pair{"success", false})
					assertMarkerEqual(t, markers[11], "get_id_list_sources", "process", "end", Pair{"success", true}, Pair{"idListCount", float64(1)})
				}
			},
		})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: false,
			DisableApiDiagnostics:  true,
		},
		ConfigSyncInterval: time.Millisecond * 900,
		IDListSyncInterval: time.Millisecond * 1000,
		LoggingInterval:    time.Millisecond * 1100,
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 1
	})
}

func TestApiCallDiagnostics(t *testing.T) {
	var events events
	testServer := getTestServer(
		testServerOptions{
			dcsOnline: true,
			onLogEvent: func(newEvents []map[string]interface{}) {
				events = newEvents
			},
		})

	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  false,
		},
	}
	InitializeWithOptions("secret-key", options)
	user := User{UserID: "123"}
	CheckGate(user, "non_existent_gate")
	GetConfig(user, "non_existent_config")
	GetExperiment(user, "non_existent_experiment")
	GetLayer(user, "non_existent_layer")
	ShutdownAndDangerouslyClearInstance()

	markers := extractMarkers(events, 3) // 3 exposure events, api diagnostics

	if len(markers) != 8 {
		t.Errorf("Expected %d markers but got %d", 8, len(markers))
	}

	assertMarkerEqual(t, markers[0], "check_gate", "", "start")
	assertMarkerEqual(t, markers[1], "check_gate", "", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[2], "get_config", "", "start")
	assertMarkerEqual(t, markers[3], "get_config", "", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[4], "get_config", "", "start")
	assertMarkerEqual(t, markers[5], "get_config", "", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[6], "get_layer", "", "start")
	assertMarkerEqual(t, markers[7], "get_layer", "", "end", Pair{"success", true})
}

func TestBootstrapDiagnostics(t *testing.T) {
	var events events
	testServer := getTestServer(
		testServerOptions{
			dcsOnline: true,
			onLogEvent: func(newEvents []map[string]interface{}) {
				events = newEvents
			},
		})

	defer testServer.Close()

	bytes, _ := os.ReadFile("download_config_specs.json")

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
			DisableApiDiagnostics:  false,
		},
		BootstrapValues: string(bytes),
	}
	InitializeWithOptions("secret-key", options)
	ShutdownAndDangerouslyClearInstance()

	if len(events) != 1 {
		t.Errorf("Expected 1 diagnostics events, received %d", len(events))
	}

	markers := extractMarkers(events, 0)

	if len(markers) != 12 {
		t.Errorf("Expected %d markers but got %d", 12, len(markers))
	}

	assertMarkerEqual(t, markers[0], "overall", "", "start")
	assertMarkerEqual(t, markers[1], "bootstrap", "process", "start")
	assertMarkerEqual(t, markers[2], "bootstrap", "process", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[3], "get_id_list_sources", "network_request", "start")
	assertMarkerEqual(t, markers[4], "get_id_list_sources", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[5], "get_id_list_sources", "process", "start", Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[6], "get_id_list", "network_request", "start")
	assertMarkerEqual(t, markers[7], "get_id_list", "network_request", "end", Pair{"statusCode", float64(200)})
	assertMarkerEqual(t, markers[8], "get_id_list", "process", "start")
	assertMarkerEqual(t, markers[9], "get_id_list", "process", "end", Pair{"success", false})
	assertMarkerEqual(t, markers[10], "get_id_list_sources", "process", "end", Pair{"success", true}, Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[11], "overall", "", "end", Pair{"success", true})
}

func TestDiagnosticsGetCleared(t *testing.T) {
	var mu sync.Mutex
	count := 0
	testServer := getTestServer(testServerOptions{
		dcsOnline: true,
		onLogEvent: func(events []map[string]interface{}) {
			mu.Lock()
			defer mu.Unlock()
			count += 1

			if count == 1 {
				if len(events) != 2 { // initialize & config_sync
					t.Errorf("Expected 2 diagnostics events, received %d", len(events))
				}

				metadata, ok := events[1]["metadata"].(map[string]interface{})
				if !ok {
					t.Error("Expected metadata to be of type map[string]interface{}")
				}
				if metadata["context"] != "config_sync" {
					t.Errorf("Expected marker context to be 'config_sync' but got %s", metadata["context"])
				}
				markers := extractMarkers(events, 1)

				if len(markers) != 12 {
					t.Errorf("Expected %d markers but got %d", 12, len(markers))
				}
			}

			if count == 2 {
				if len(events) != 1 {
					t.Errorf("Expected 1 diagnostics events, received %d", len(events))
				}

				metadata, ok := events[0]["metadata"].(map[string]interface{})
				if !ok {
					t.Error("Expected metadata to be of type map[string]interface{}")
				}
				markers := extractMarkers(events, 0)

				if metadata["context"] != "config_sync" {
					t.Errorf("Expected marker context to be 'config_sync' but got %s", metadata["context"])
				}

				if len(markers) != 12 {
					t.Errorf("Expected %d markers but got %d", 12, len(markers))
				}
			}
		}})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
			DisableApiDiagnostics:  false,
		},
		ConfigSyncInterval: time.Millisecond * 900,
		IDListSyncInterval: time.Millisecond * 1000,
		LoggingInterval:    time.Millisecond * 1100,
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 2
	})
}

func TestDiagnosticsSampling(t *testing.T) {
	var events events
	var mu sync.RWMutex

	testServer := getTestServer(testServerOptions{
		dcsOnline: true,
		onLogEvent: func(newEvents []map[string]interface{}) {
			mu.Lock()
			events = append(events, newEvents...)
			mu.Unlock()
		},
		withSampling: true,
	})

	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
			DisableApiDiagnostics:  false,
		},
		ConfigSyncInterval: time.Millisecond * 99999,
		IDListSyncInterval: time.Millisecond * 99999,
		LoggingInterval:    time.Millisecond * 99999,
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	for i := 1; i <= 10; i++ {
		instance.evaluator.store.fetchConfigSpecsFromServer(false)
		instance.logger.flush(false)
	}
	numEvents := len(events)
	if !(numEvents > 0 && numEvents < 10) {
		t.Errorf("Expected between %d and %d events, received %d", 0, 10, numEvents)
	}

	events = nil

	for i := 1; i <= 10; i++ {
		instance.evaluator.store.fetchIDListsFromServer()
		instance.logger.flush(false)
	}
	numEvents = len(events)
	if !(numEvents > 0 && numEvents < 10) {
		t.Errorf("Expected between %d and %d events, received %d", 0, 10, numEvents)
	}
}

func TestDiagnosticsClearMarkers(t *testing.T) {
	var events events
	testServer := getTestServer(
		testServerOptions{
			dcsOnline: true,
			onLogEvent: func(newEvents []map[string]interface{}) {
				events = append(events, newEvents...)
			},
			withSampling: true,
		})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
			DisableApiDiagnostics:  false,
		},
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()
	for i := 1; i <= 10; i++ {
		instance.evaluator.store.fetchConfigSpecsFromServer(false)
		instance.logger.flush(false)
	}

	initMarkersLen := len(instance.diagnostics.initDiagnostics.markers)
	if initMarkersLen > 0 {
		t.Errorf("Expected no markers, received %d", initMarkersLen)
	}
	apiMarkersLen := len(instance.diagnostics.initDiagnostics.markers)
	if apiMarkersLen > 0 {
		t.Errorf("Expected no markers, received %d", apiMarkersLen)
	}
	configSyncMarkersLen := len(instance.diagnostics.initDiagnostics.markers)
	if apiMarkersLen > 0 {
		t.Errorf("Expected no markers, received %d", configSyncMarkersLen)
	}
}

func TestDiagnosticsMaxMarkers(t *testing.T) {
	var events events
	testServer := getTestServer(
		testServerOptions{
			dcsOnline: true,
			onLogEvent: func(newEvents []map[string]interface{}) {
				events = newEvents
			},
		})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  false,
		},
	}
	InitializeWithOptions("secret-key", options)
	user := User{UserID: "123"}
	for i := 0; i < 10; i++ {
		CheckGate(user, "non_existent_gate")
		GetConfig(user, "non_existent_config")
		GetExperiment(user, "non_existent_experiment")
		GetLayer(user, "non_existent_layer")
	}

	ShutdownAndDangerouslyClearInstance()

	markers := extractMarkers(events, 30) // 30 exposure events, api diagnostics
	lenMarkers := len(markers)

	if lenMarkers > MaxMarkerSize || lenMarkers < 0 {
		t.Errorf("Expected at most %d markers but got %d", MaxMarkerSize, len(markers))
	}
}

func TestDisableDiagnostics(t *testing.T) {
	var events events
	testServer := getTestServer(testServerOptions{
		dcsOnline: true,
		onLogEvent: func(newEvents []map[string]interface{}) {
			events = newEvents

		},
	})

	defer testServer.Close()
	user := User{UserID: "123"}
	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  true,
		},
	}
	InitializeWithOptions("secret-key", options)
	CheckGate(user, "always_on_gate")
	GetConfig(user, "non_existent_config")
	GetExperiment(user, "non_existent_experiment")
	GetLayer(user, "non_existent_layer")
	instance.evaluator.store.fetchConfigSpecsFromServer(false)
	instance.logger.flush(true)
	defer ShutdownAndDangerouslyClearInstance()

	logEventsLen := len(events)
	if logEventsLen != 3 { // 3 Exposure events
		t.Errorf("Diagnostics logged to endpoints %d", logEventsLen)
	}

	initMarkersLen := len(instance.diagnostics.initDiagnostics.markers)
	if initMarkersLen > 0 {
		t.Errorf("Expected no markers, received %d", initMarkersLen)
	}
	apiMarkersLen := len(instance.diagnostics.initDiagnostics.markers)
	if apiMarkersLen > 0 {
		t.Errorf("Expected no markers, received %d", apiMarkersLen)
	}
	configSyncMarkersLen := len(instance.diagnostics.initDiagnostics.markers)
	if configSyncMarkersLen > 0 {
		t.Errorf("Expected no markers, received %d", configSyncMarkersLen)
	}
}

func assertMarkerEqual(t *testing.T, marker map[string]interface{}, key string, step string, action string, tags ...Pair) {
	if marker["key"] != key && !(marker["key"] == nil && key == "") {
		t.Errorf("Expected key to be %s but got %s", key, marker["key"])
	}
	if marker["step"] != step && !(marker["step"] == nil && step == "") {
		t.Errorf("Expected step to be %s but got %s", step, marker["step"])
	}
	if marker["action"] != action && !(marker["action"] == nil && action == "") {
		t.Errorf("Expected action to be %s but got %s", action, marker["action"])
	}
	for _, tag := range tags {
		if marker[tag.A] != tag.B && !(marker[tag.A] == nil && tag.B == "") {
			t.Errorf("Expected %s to be %+v but got %+v", tag.A, tag.B, marker[tag.A])
		}
	}
	if marker["timestamp"] == nil || marker["timestamp"] == 0 {
		t.Error("Expected timestamp to be non zero")
	}
}

func extractMarkers(events events, index int) []map[string]interface{} {
	initializeDiagnostics, ok := events[index]["metadata"].(map[string]interface{})
	if !ok {
		initializeDiagnostics = make(map[string]interface{})
	}
	markers, ok := initializeDiagnostics["markers"].([]interface{})
	if !ok {
		markers = make([]interface{}, 0)
	}

	details := make([]map[string]interface{}, len(markers))
	for i, m := range markers {
		details[i], ok = m.(map[string]interface{})
		if !ok {
			details[i] = make(map[string]interface{})
		}
	}

	return details
}

func waitForCondition(t *testing.T, condition func() bool) {
	waitForConditionWithMessage(t, condition, "Timeout Expired")
}

func waitForConditionWithMessage(t *testing.T, condition func() bool, errorMsg string) {
	timeout := 5000 * time.Millisecond
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond) // Adjust the polling interval as needed
	}

	t.Errorf(errorMsg)
}
