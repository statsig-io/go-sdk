package statsig

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBootstrapWithAdapter(t *testing.T) {
	events := []Event{}
	dcs_bytes, _ := os.ReadFile("download_config_specs.json")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "log_event") {
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
	dataAdapter := dataAdapterExample{store: make(map[string]string)}
	dataAdapter.Initialize()
	defer dataAdapter.Shutdown()
	dataAdapter.Set(CONFIG_SPECS_KEY, string(dcs_bytes))
	options := &Options{
		DataAdapter:          dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", options)
	user := User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}

	t.Run("able to fetch data from adapter and populate store without network", func(t *testing.T) {
		value := CheckGate(user, "always_on_gate")
		if !value {
			t.Errorf("Expected gate to return true")
		}
		config := GetConfig(user, "test_config")
		if config.GetString("string", "") != "statsig" {
			t.Errorf("Expected config to return statsig")
		}
		layer := GetLayer(user, "a_layer")
		if layer.GetString("experiment_param", "") != "control" {
			t.Errorf("Expected layer param to return control")
		}
		ShutdownAndDangerouslyClearInstance() // shutdown here to flush event queue
		if len(events) != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", len(events))
		}
		for _, event := range events {
			if event.Metadata["reason"] != string(reasonDataAdapter) {
				t.Errorf("Expected init reason to be %s", reasonDataAdapter)
			}
		}
	})
}

func TestSaveToAdapter(t *testing.T) {
	bytes, _ := os.ReadFile("download_config_specs.json")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			_ = json.NewDecoder(req.Body).Decode(&in)
			_, _ = res.Write(bytes)
		}
	}))
	dataAdapter := dataAdapterExample{store: make(map[string]string)}
	options := &Options{
		DataAdapter:          dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	t.Run("updates adapter with newer values from network", func(t *testing.T) {
		specString := dataAdapter.Get(CONFIG_SPECS_KEY)
		specs := downloadConfigSpecResponse{}
		err := json.Unmarshal([]byte(specString), &specs)
		if err != nil {
			t.Errorf("Error parsing data adapter values")
		}
		if !contains_spec(specs.FeatureGates, "always_on_gate", "feature_gate") {
			t.Errorf("Expected data adapter to have downloaded gates")
		}
		if !contains_spec(specs.DynamicConfigs, "test_config", "dynamic_config") {
			t.Errorf("Expected data adapter to have downloaded configs")
		}
		if !contains_spec(specs.LayerConfigs, "a_layer", "dynamic_config") {
			t.Errorf("Expected data adapter to have downloaded layers")
		}
	})
}

func TestAdapterWithPolling(t *testing.T) {
	bytes, _ := os.ReadFile("download_config_specs.json")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			_ = json.NewDecoder(req.Body).Decode(&in)
			_, _ = res.Write(bytes)
		}
	}))
	dataAdapter := dataAdapterWithPollingExample{store: make(map[string]string)}
	options := &Options{
		DataAdapter:          &dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		ConfigSyncInterval:   100 * time.Millisecond,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()
	user := User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}
	t.Run("updating adapter also updates statsig store", func(t *testing.T) {
		value := CheckGate(user, "always_on_gate")
		if !value {
			t.Errorf("Expected gate to return true")
		}
		dataAdapter.clearStore(CONFIG_SPECS_KEY)
		time.Sleep(100 * time.Millisecond)
		value = CheckGate(user, "always_on_gate")
		if value {
			t.Errorf("Expected gate to return false")
		}
	})
}

func TestIncorrectlyImplementedAdapter(t *testing.T) {
	events := []Event{}
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			bytes, _ := os.ReadFile("download_config_specs.json")
			_ = json.NewDecoder(req.Body).Decode(&in)
			_, _ = res.Write(bytes)
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
	dataAdapter := brokenDataAdapterExample{}
	options := &Options{
		DataAdapter:          dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	stderrLogs := swallow_stderr(func() {
		InitializeWithOptions("secret-key", options)
	})
	if stderrLogs == "" {
		t.Errorf("Expected output to stderr")
	}
	user := User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}

	t.Run("recover and finish initialize if adapter panics", func(t *testing.T) {
		value := CheckGate(user, "always_on_gate")
		if !value {
			t.Errorf("Expected gate to return true")
		}
		config := GetConfig(user, "test_config")
		if config.GetString("string", "") != "statsig" {
			t.Errorf("Expected config to return statsig")
		}
		layer := GetLayer(user, "a_layer")
		if layer.GetString("experiment_param", "") != "control" {
			t.Errorf("Expected layer param to return control")
		}
		ShutdownAndDangerouslyClearInstance() // shutdown here to flush event queue
		if len(events) != 3 {
			t.Errorf("Should receive exactly 3 log_event. Got %d", len(events))
		}
		for _, event := range events {
			if event.Metadata["reason"] != string(reasonNetwork) {
				t.Errorf("Expected init reason to be %s", reasonNetwork)
			}
		}
	})
}

func swallow_stderr(task func()) string {
	stderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	task()
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stderr = stderr
	return buf.String()
}

func contains_spec(specs []configSpec, name string, specType string) bool {
	for _, e := range specs {
		if e.Name == name && e.Type == specType {
			return true
		}
	}
	return false
}
