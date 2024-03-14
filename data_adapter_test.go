package statsig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBootstrapWithAdapter(t *testing.T) {
	var events []Event
	dcsBytes, _ := os.ReadFile("download_config_specs.json")
	idlistsBytes, _ := os.ReadFile("test_data/get_id_lists.json")
	idlistBytes, _ := os.ReadFile("test_data/list_1.txt")
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
	dataAdapter.Set(configSpecsKey, string(dcsBytes))
	dataAdapter.Set(idListsKey, string(idlistsBytes))
	dataAdapter.Set(fmt.Sprintf("%s::%s", idListsKey, "list_1"), string(idlistBytes))
	options := &Options{
		DataAdapter:          &dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
	}

	t.Run("able to fetch config spec data from adapter and populate store without network", func(t *testing.T) {
		InitializeWithOptions("secret-key", options)
		user := User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}
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

	t.Run("able to fetch id list data from adapter and populate store without network", func(t *testing.T) {
		InitializeWithOptions("secret-key", options)
		defer ShutdownAndDangerouslyClearInstance()
		user := User{UserID: "abc"}
		value := CheckGate(user, "on_for_id_list")
		if !value {
			t.Errorf("Expected gate to return true")
		}
	})
}

func TestSaveToAdapter(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			bytes, _ := os.ReadFile("download_config_specs.json")
			_, _ = res.Write(bytes)
		}
		if strings.Contains(req.URL.Path, "get_id_lists") {
			baseURL := "http://" + req.Host
			r := map[string]idList{
				"list_1": {Name: "list_1", Size: 20, URL: baseURL + "/list_1", CreationTime: 0, FileID: "123"},
			}
			v, _ := json.Marshal(r)
			_, _ = res.Write(v)
		}
		if strings.Contains(req.URL.Path, "list_1") {
			_, _ = res.Write([]byte("+ungWv48B\n+Ngi8oeRO\n"))
		}
	}))
	dataAdapter := dataAdapterExample{store: make(map[string]string)}
	options := &Options{
		DataAdapter:          &dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
		IDListSyncInterval:   100 * time.Millisecond,
		ConfigSyncInterval:   100 * time.Millisecond,
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	t.Run("updates adapter with newer config spec values from network", func(t *testing.T) {
		waitForCondition(t, func() bool {
			return dataAdapter.Get(configSpecsKey) != ""
		})
		specString := dataAdapter.Get(configSpecsKey)
		specs := downloadConfigSpecResponse{}
		err := json.Unmarshal([]byte(specString), &specs)
		if err != nil {
			t.Errorf("Error parsing data adapter values")
		}
		if !containsSpec(specs.FeatureGates, "always_on_gate", "feature_gate") {
			t.Errorf("Expected data adapter to have downloaded gates")
		}
		if !containsSpec(specs.DynamicConfigs, "test_config", "dynamic_config") {
			t.Errorf("Expected data adapter to have downloaded configs")
		}
		if !containsSpec(specs.LayerConfigs, "a_layer", "dynamic_config") {
			t.Errorf("Expected data adapter to have downloaded layers")
		}
	})

	t.Run("updates adapter with newer id list values from network", func(t *testing.T) {
		waitForCondition(t, func() bool {
			return dataAdapter.Get(idListsKey) != ""
		})
		idListsString := dataAdapter.Get(idListsKey)
		list1String := dataAdapter.Get(fmt.Sprintf("%s::%s", idListsKey, "list_1"))
		list1Bytes := []byte(list1String)
		var idLists map[string]idList
		err := json.Unmarshal([]byte(idListsString), &idLists)
		if err != nil {
			t.Errorf("Error parsing data adapter values")
		}
		if len(idLists) != 1 {
			t.Errorf("Expected data adapter to have list_1")
		}
		if idLists["list_1"].Size != 20 {
			t.Errorf("Expected list_1 to have size 20, received %d", idLists["list_1"].Size)
		}
		if idLists["list_1"].FileID != "123" {
			t.Errorf("Expected list_1 to have file ID 123")
		}
		if len(list1Bytes) != 20 {
			t.Errorf("Expected list_1 to have 20 bytes, received %d", len(list1Bytes))
		}
		if list1String != "+ungWv48B\n+Ngi8oeRO\n" {
			t.Errorf("Expected list_1 to contain ids: ungWv48B, Ngi8oeRO")
		}
	})
}

func TestAdapterWithPolling(t *testing.T) {
	dcsBytes, _ := os.ReadFile("download_config_specs.json")
	idlistsBytes, _ := os.ReadFile("test_data/get_id_lists.json")
	idlistBytes, _ := os.ReadFile("test_data/list_1.txt")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			_, _ = res.Write(dcsBytes)
		}
	}))
	dataAdapter := dataAdapterWithPollingExample{store: make(map[string]string)}
	dataAdapter.Set(configSpecsKey, string(dcsBytes))
	dataAdapter.Set(idListsKey, string(idlistsBytes))
	dataAdapter.Set(fmt.Sprintf("%s::%s", idListsKey, "list_1"), string(idlistBytes))
	options := &Options{
		DataAdapter:          &dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		ConfigSyncInterval:   100 * time.Millisecond,
		IDListSyncInterval:   100 * time.Millisecond,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	t.Run("updating adapter also updates statsig store", func(t *testing.T) {
		user := User{UserID: "abc"}
		value := CheckGate(user, "on_for_id_list")
		if !value {
			t.Errorf("Expected on_for_id_list to return true")
		}
		idlistsUpdatedBytes, _ := os.ReadFile("test_data/get_id_lists_updated.json")
		idlistUpdatedBytes, _ := os.ReadFile("test_data/list_1_updated.txt")
		dataAdapter.Set(fmt.Sprintf("%s::%s", idListsKey, "list_1"), string(idlistUpdatedBytes))
		dataAdapter.Set(idListsKey, string(idlistsUpdatedBytes))
		waitForConditionWithMessage(t, func() bool {
			return !CheckGate(user, "on_for_id_list")
		}, "Expected on_for_id_list to return false")

		user = User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}
		value = CheckGate(user, "always_on_gate")
		if !value {
			t.Errorf("Expected always_on_gate to return true")
		}
		dataAdapter.clearStore(configSpecsKey)
		waitForConditionWithMessage(t, func() bool {
			return !CheckGate(user, "always_on_gate")
		}, "Expected always_on_gate to return false")
	})
}

func TestIncorrectlyImplementedAdapter(t *testing.T) {
	var events []Event
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			bytes, _ := os.ReadFile("download_config_specs.json")
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
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
	}
	stderrLogs := swallowStderr(func() {
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

func swallowStderr(task func()) string {
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

func containsSpec(specs []configSpec, name string, specType string) bool {
	for _, e := range specs {
		if e.Name == name && e.Type == specType {
			return true
		}
	}
	return false
}
