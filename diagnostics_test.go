package statsig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type Pair struct {
	A string
	B interface{}
}

type Events []map[string]interface{}

func TestInitDiagnostics(t *testing.T) {
	var events Events
	testServer := getTestServer(true, func(newEvents Events) {
		events = newEvents
	})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
		},
	}
	InitializeWithOptions("secret-key", options)
	shutDownAndClearInstance()

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
	testServer := getTestServer(true, func(events Events) {
		mu.Lock()
		defer mu.Unlock()
		count += 1

		if count == 1 {
			if len(events) != 2 {
				t.Errorf("Expected 2 diagnostics events, received %d", len(events))
			}

			markers := extractMarkers(events, 1)

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
	})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
		},
		ConfigSyncInterval: time.Millisecond * 900,
		IDListSyncInterval: time.Millisecond * 1000,
		LoggingInterval:    time.Millisecond * 1100,
	}
	InitializeWithOptions("secret-key", options)
	defer shutDownAndClearInstance()

	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 1
	})
}

func TestBootstrapDiagnostics(t *testing.T) {
	var events Events
	testServer := getTestServer(true, func(newEvents Events) {
		events = newEvents
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
		},
		BootstrapValues: string(bytes),
	}
	InitializeWithOptions("secret-key", options)
	shutDownAndClearInstance()

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
	testServer := getTestServer(true, func(events Events) {
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
	})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
		},
		ConfigSyncInterval: time.Millisecond * 900,
		IDListSyncInterval: time.Millisecond * 1000,
		LoggingInterval:    time.Millisecond * 1100,
	}
	InitializeWithOptions("secret-key", options)
	defer shutDownAndClearInstance()

	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 2
	})
}

func getTestIDListServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "my_id_list") {
			res.WriteHeader(http.StatusOK)
			response, _ := json.Marshal("+asdfcd")
			_, _ = res.Write(response)
		}
	}))
}

func getTestServer(dcsOnline bool, onLog func(events Events)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("x-statsig-region", "az-westus-2")
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
			res.WriteHeader(http.StatusOK)
			type requestInput struct {
				Events          []map[string]interface{} `json:"events"`
				StatsigMetadata statsigMetadata          `json:"statsigMetadata"`
			}
			input := &requestInput{}
			defer req.Body.Close()
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(req.Body)

			_ = json.Unmarshal(buf.Bytes(), &input)

			if onLog != nil {
				onLog(input.Events)
			}
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			res.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(map[string]map[string]interface{}{
				"my_id_list": {
					"name":         "my_id_list",
					"size":         1,
					"url":          fmt.Sprintf("%s/my_id_list", getTestIDListServer().URL),
					"creationTime": 1,
					"fileID":       "a_file_id",
				},
			})
			_, _ = res.Write(response)
		}
	}))
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

func extractMarkers(events []map[string]interface{}, index int) []map[string]interface{} {
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
	timeout := 2000 * time.Millisecond
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond) // Adjust the polling interval as needed
	}

	t.Errorf("Timeout Expired")
}
