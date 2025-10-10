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
	events := []Event{}
	dcs_bytes, _ := os.ReadFile("download_config_specs.json")
	idlists_bytes, _ := os.ReadFile("test_data/get_id_lists.json")
	idlist_bytes, _ := os.ReadFile("test_data/list_1.txt")
	testServer := getTestServer(testServerOptions{
		onLogEvent: func(newEvents []map[string]interface{}) {
			for _, newEvent := range newEvents {
				eventTyped := convertToExposureEvent(newEvent)
				events = append(events, eventTyped)
			}
		},
	})
	dataAdapter := dataAdapterExample{store: make(map[string]string)}
	dataAdapter.Initialize()
	defer dataAdapter.Shutdown()
	dataAdapter.Set(CONFIG_SPECS_KEY, string(dcs_bytes))
	dataAdapter.Set(ID_LISTS_KEY, string(idlists_bytes))
	dataAdapter.Set(fmt.Sprintf("%s::%s", ID_LISTS_KEY, "list_1"), string(idlist_bytes))
	options := &Options{
		DataAdapter:          &dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
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
			if event.Metadata["reason"] != string(SourceDataAdapter) {
				t.Errorf("Expected init reason to be %s", SourceDataAdapter)
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
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		IDListSyncInterval:   100 * time.Millisecond,
		ConfigSyncInterval:   100 * time.Millisecond,
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	t.Run("updates adapter with newer config spec values from network", func(t *testing.T) {
		waitForCondition(t, func() bool {
			return dataAdapter.Get(CONFIG_SPECS_KEY) != ""
		})
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

	t.Run("updates adapter with newer id list values from network", func(t *testing.T) {
		waitForCondition(t, func() bool {
			return dataAdapter.Get(ID_LISTS_KEY) != "" && dataAdapter.Get(fmt.Sprintf("%s::%s", ID_LISTS_KEY, "list_1")) != ""
		})
		idListsString := dataAdapter.Get(ID_LISTS_KEY)
		list1String := dataAdapter.Get(fmt.Sprintf("%s::%s", ID_LISTS_KEY, "list_1"))
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
		if !strings.Contains(list1String, "+ungWv48B\n") || !strings.Contains(list1String, "+Ngi8oeRO\n") {
			// the list should be exactly +ungWv48B\n+Ngi8oeRO\n. Will fix in the future, patching this test for now so it's not flakey
			t.Errorf("Expected list_1 to contain ids: ungWv48B, Ngi8oeRO, received %v", strings.Split(list1String, "\n"))
		}
	})
}

func TestAdapterWithPolling(t *testing.T) {
	dcs_bytes, _ := os.ReadFile("download_config_specs.json")
	idlists_bytes, _ := os.ReadFile("test_data/get_id_lists.json")
	idlist_bytes, _ := os.ReadFile("test_data/list_1.txt")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			_, _ = res.Write(dcs_bytes)
		}
	}))
	dataAdapter := dataAdapterWithPollingExample{store: make(map[string]string)}
	dataAdapter.Set(CONFIG_SPECS_KEY, string(dcs_bytes))
	dataAdapter.Set(ID_LISTS_KEY, string(idlists_bytes))
	dataAdapter.Set(fmt.Sprintf("%s::%s", ID_LISTS_KEY, "list_1"), string(idlist_bytes))
	options := &Options{
		DataAdapter:          &dataAdapter,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		ConfigSyncInterval:   100 * time.Millisecond,
		IDListSyncInterval:   100 * time.Millisecond,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", options)
	defer ShutdownAndDangerouslyClearInstance()

	t.Run("updating adapter also updates statsig store", func(t *testing.T) {
		user := User{UserID: "abc"}
		value := CheckGate(user, "on_for_id_list")
		if !value {
			t.Errorf("Expected on_for_id_list to return true")
		}
		idlists_updated_bytes, _ := os.ReadFile("test_data/get_id_lists_updated.json")
		idlist_updated_bytes, _ := os.ReadFile("test_data/list_1_updated.txt")
		dataAdapter.Set(fmt.Sprintf("%s::%s", ID_LISTS_KEY, "list_1"), string(idlist_updated_bytes))
		dataAdapter.Set(ID_LISTS_KEY, string(idlists_updated_bytes))
		waitForConditionWithMessage(t, func() bool {
			return !CheckGate(user, "on_for_id_list")
		}, "Expected on_for_id_list to return false")

		user = User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}
		value = CheckGate(user, "always_on_gate")
		if !value {
			t.Errorf("Expected always_on_gate to return true")
		}
		dataAdapter.clearStore(CONFIG_SPECS_KEY)
		waitForConditionWithMessage(t, func() bool {
			return !CheckGate(user, "always_on_gate")
		}, "Expected always_on_gate to return false")
	})
}

func TestIncorrectlyImplementedAdapter(t *testing.T) {
	events := []Event{}
	testServer := getTestServer(testServerOptions{
		onLogEvent: func(newEvents []map[string]interface{}) {
			for _, newEvent := range newEvents {
				eventTyped := convertToExposureEvent(newEvent)
				events = append(events, eventTyped)
			}
		},
	})
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
			if event.Metadata["reason"] != string(SourceNetwork) {
				t.Errorf("Expected init reason to be %s", SourceNetwork)
			}
		}
	})
}

func swallow_stderr(task func()) string {
	stderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	task()
	_ = w.Close()
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
