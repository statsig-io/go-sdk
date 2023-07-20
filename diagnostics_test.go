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

func getTestIDListServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "my_id_list") {
			res.WriteHeader(http.StatusOK)
			response, _ := json.Marshal("+asdfcd")
			_, _ = res.Write(response)
		}
	}))
}

func getTestServer(dcsOnline bool, events *[]Event, mu *sync.RWMutex) *httptest.Server {
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
				Events          []Event         `json:"events"`
				StatsigMetadata statsigMetadata `json:"statsigMetadata"`
			}
			input := &requestInput{}
			defer req.Body.Close()
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(req.Body)

			_ = json.Unmarshal(buf.Bytes(), &input)
			mu.Lock()
			*events = input.Events
			mu.Unlock()
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

type Pair struct {
	A string
	B interface{}
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

func TestInitDiagnostics(t *testing.T) {
	var mu sync.RWMutex
	events := []Event{}
	testServer := getTestServer(true, &events, &mu)

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

	eventsCopy := copyEvents(&events, &mu)
	if len(eventsCopy) != 1 {
		t.Errorf("Expected 1 diagnostics events, received %d", len(eventsCopy))
	}
	initializeDiagnostics := eventsCopy[0].Metadata
	markers, ok := initializeDiagnostics["markers"].([]interface{})
	if !ok || initializeDiagnostics["context"] != "initialize" {
		t.Errorf("Expected marker context to be 'initialize' but got %s", initializeDiagnostics["context"])
	}
	if len(markers) != 14 {
		t.Errorf("Expected %d markers but got %d", 14, len(markers))
	}
	assertMarkerEqual(t, markers[0].(map[string]interface{}), "overall", "", "start")
	assertMarkerEqual(t, markers[1].(map[string]interface{}), "download_config_specs", "network_request", "start")
	assertMarkerEqual(t, markers[2].(map[string]interface{}), "download_config_specs", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[3].(map[string]interface{}), "download_config_specs", "process", "start")
	assertMarkerEqual(t, markers[4].(map[string]interface{}), "download_config_specs", "process", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[5].(map[string]interface{}), "get_id_list_sources", "network_request", "start")
	assertMarkerEqual(t, markers[6].(map[string]interface{}), "get_id_list_sources", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[7].(map[string]interface{}), "get_id_list_sources", "process", "start", Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[8].(map[string]interface{}), "get_id_list", "network_request", "start")
	assertMarkerEqual(t, markers[9].(map[string]interface{}), "get_id_list", "network_request", "end", Pair{"statusCode", float64(200)})
	assertMarkerEqual(t, markers[10].(map[string]interface{}), "get_id_list", "process", "start")
	assertMarkerEqual(t, markers[11].(map[string]interface{}), "get_id_list", "process", "end", Pair{"success", false})
	assertMarkerEqual(t, markers[12].(map[string]interface{}), "get_id_list_sources", "process", "end", Pair{"success", true}, Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[13].(map[string]interface{}), "overall", "", "end", Pair{"success", true})
}

func TestConfigSyncDiagnostics(t *testing.T) {
	var mu sync.RWMutex
	events := []Event{}
	testServer := getTestServer(true, &events, &mu)

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
		},
		ConfigSyncInterval: time.Millisecond * 90,
		IDListSyncInterval: time.Millisecond * 100,
		LoggingInterval:    time.Millisecond * 110,
	}
	InitializeWithOptions("secret-key", options)
	defer shutDownAndClearInstance()

	time.Sleep(120 * time.Millisecond)

	eventsCopy := copyEvents(&events, &mu)
	if len(eventsCopy) != 2 {
		t.Errorf("Expected 2 diagnostics events, received %d", len(eventsCopy))
	}

	configSyncDiagnostics := eventsCopy[1].Metadata
	markers, ok := configSyncDiagnostics["markers"].([]interface{})
	if !ok || configSyncDiagnostics["context"] != "config_sync" {
		t.Errorf("Expected marker context to be 'config_sync' but got %s", configSyncDiagnostics["context"])
	}
	if len(markers) != 12 {
		t.Errorf("Expected %d markers but got %d", 12, len(markers))
	}
	assertMarkerEqual(t, markers[0].(map[string]interface{}), "download_config_specs", "network_request", "start")
	assertMarkerEqual(t, markers[1].(map[string]interface{}), "download_config_specs", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[2].(map[string]interface{}), "download_config_specs", "process", "start")
	assertMarkerEqual(t, markers[3].(map[string]interface{}), "download_config_specs", "process", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[4].(map[string]interface{}), "get_id_list_sources", "network_request", "start")
	assertMarkerEqual(t, markers[5].(map[string]interface{}), "get_id_list_sources", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[6].(map[string]interface{}), "get_id_list_sources", "process", "start", Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[7].(map[string]interface{}), "get_id_list", "network_request", "start")
	assertMarkerEqual(t, markers[8].(map[string]interface{}), "get_id_list", "network_request", "end", Pair{"statusCode", float64(200)})
	assertMarkerEqual(t, markers[9].(map[string]interface{}), "get_id_list", "process", "start")
	assertMarkerEqual(t, markers[10].(map[string]interface{}), "get_id_list", "process", "end", Pair{"success", false})
	assertMarkerEqual(t, markers[11].(map[string]interface{}), "get_id_list_sources", "process", "end", Pair{"success", true}, Pair{"idListCount", float64(1)})
}

func TestBootstrapDiagnostics(t *testing.T) {
	var mu sync.RWMutex
	events := []Event{}
	testServer := getTestServer(true, &events, &mu)
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

	eventsCopy := copyEvents(&events, &mu)
	if len(eventsCopy) != 1 {
		t.Errorf("Expected 1 diagnostics events, received %d", len(eventsCopy))
	}

	bootstrapDiagnostics := eventsCopy[0].Metadata
	markers, ok := bootstrapDiagnostics["markers"].([]interface{})
	if !ok || bootstrapDiagnostics["context"] != "initialize" {
		t.Errorf("Expected marker context to be 'initialize' but got %s", bootstrapDiagnostics["context"])
	}
	if len(markers) != 12 {
		t.Errorf("Expected %d markers but got %d", 12, len(markers))
	}
	assertMarkerEqual(t, markers[0].(map[string]interface{}), "overall", "", "start")
	assertMarkerEqual(t, markers[1].(map[string]interface{}), "bootstrap", "process", "start")
	assertMarkerEqual(t, markers[2].(map[string]interface{}), "bootstrap", "process", "end", Pair{"success", true})
	assertMarkerEqual(t, markers[3].(map[string]interface{}), "get_id_list_sources", "network_request", "start")
	assertMarkerEqual(t, markers[4].(map[string]interface{}), "get_id_list_sources", "network_request", "end", Pair{"success", true}, Pair{"statusCode", float64(200)}, Pair{"sdkRegion", "az-westus-2"})
	assertMarkerEqual(t, markers[5].(map[string]interface{}), "get_id_list_sources", "process", "start", Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[6].(map[string]interface{}), "get_id_list", "network_request", "start")
	assertMarkerEqual(t, markers[7].(map[string]interface{}), "get_id_list", "network_request", "end", Pair{"statusCode", float64(200)})
	assertMarkerEqual(t, markers[8].(map[string]interface{}), "get_id_list", "process", "start")
	assertMarkerEqual(t, markers[9].(map[string]interface{}), "get_id_list", "process", "end", Pair{"success", false})
	assertMarkerEqual(t, markers[10].(map[string]interface{}), "get_id_list_sources", "process", "end", Pair{"success", true}, Pair{"idListCount", float64(1)})
	assertMarkerEqual(t, markers[11].(map[string]interface{}), "overall", "", "end", Pair{"success", true})
}

func TestDiagnosticsGetCleared(t *testing.T) {
	var mu sync.RWMutex
	events := []Event{}
	testServer := getTestServer(true, &events, &mu)

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: false,
		},
		ConfigSyncInterval: time.Millisecond * 90,
		IDListSyncInterval: time.Millisecond * 100,
		LoggingInterval:    time.Millisecond * 110,
	}
	InitializeWithOptions("secret-key", options)
	defer shutDownAndClearInstance()

	// First config sync
	time.Sleep(120 * time.Millisecond)

	eventsCopy := copyEvents(&events, &mu)
	if len(eventsCopy) != 2 { // initialize & config_sync
		t.Errorf("Expected 2 diagnostics events, received %d", len(eventsCopy))
	}

	configSyncDiagnostics := eventsCopy[1].Metadata
	markers, ok := configSyncDiagnostics["markers"].([]interface{})
	if !ok || configSyncDiagnostics["context"] != "config_sync" {
		t.Errorf("Expected marker context to be 'config_sync' but got %s", configSyncDiagnostics["context"])
	}
	if len(markers) != 12 {
		t.Errorf("Expected %d markers but got %d", 12, len(markers))
	}

	// Second config sync
	time.Sleep(120 * time.Millisecond)

	eventsCopy = copyEvents(&events, &mu)
	if len(eventsCopy) != 1 {
		t.Errorf("Expected 1 diagnostics events, received %d", len(eventsCopy))
	}
	configSyncDiagnostics = eventsCopy[0].Metadata
	markers, ok = configSyncDiagnostics["markers"].([]interface{})
	if !ok || configSyncDiagnostics["context"] != "config_sync" {
		t.Errorf("Expected marker context to be 'config_sync' but got %s", configSyncDiagnostics["context"])
	}
	if len(markers) != 12 {
		t.Errorf("Expected %d markers but got %d", 12, len(markers))
	}
}

func copyEvents(events *[]Event, mu *sync.RWMutex) []Event {
	mu.RLock()
	defer mu.RUnlock()
	eventsCopy := make([]Event, len(*events))
	copy(eventsCopy, *events)
	return eventsCopy
}
