package statsig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestBootstrapWithAdapter(t *testing.T) {
	bytes, _ := os.ReadFile("download_config_specs.json")
	dataAdapter := dataAdapterExample{store: make(map[string]string)}
	dataAdapter.initialize()
	defer dataAdapter.shutdown()
	dataAdapter.set(dataAdapterKey, string(bytes))
	options := &Options{
		DataAdapter: dataAdapter,
		LocalMode:   true,
		Environment: Environment{Tier: "test"},
	}
	InitializeWithOptions("secret-key", options)
	defer shutDownAndClearInstance()
	user := User{UserID: "statsig_user", Email: "statsiguser@statsig.com"}

	t.Run("fetch from adapter when network is down", func(t *testing.T) {
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
		DataAdapter: dataAdapter,
		API:         testServer.URL,
		Environment: Environment{Tier: "test"},
	}
	InitializeWithOptions("secret-key", options)
	defer shutDownAndClearInstance()

	t.Run("updates adapter with newer values from network", func(t *testing.T) {
		specString := dataAdapter.get(dataAdapterKey)
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

func contains_spec(specs []configSpec, name string, specType string) bool {
	for _, e := range specs {
		if e.Name == name && e.Type == specType {
			return true
		}
	}
	return false
}
