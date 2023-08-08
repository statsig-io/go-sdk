package statsig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestBootstrap(t *testing.T) {
	if IsInitialized() {
		t.Errorf("expected statsig to not be initialized yet")
	}
	bytes, _ := os.ReadFile("download_config_specs.json")
	InitializeWithOptions("secret-key", &Options{
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	})
	if !IsInitialized() {
		t.Errorf("expected statsig to be initialized")
	}
	if CheckGate(User{UserID: "123"}, "always_on_gate") {
		t.Errorf("always_on_gate should return false when there is no bootstrap value")
	}
	ShutdownAndDangerouslyClearInstance()

	opt := &Options{
		BootstrapValues:      string(bytes[:]),
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opt)

	if !CheckGate(User{UserID: "123"}, "always_on_gate") {
		t.Errorf("always_on_gate should return true bootstrap value is provided")
	}
	ShutdownAndDangerouslyClearInstance()
}

func TestRulesUpdatedCallback(t *testing.T) {
	// First, verify that rules updated callback is called and returns the rules string
	bytes, _ := os.ReadFile("download_config_specs.json")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			_ = json.NewDecoder(req.Body).Decode(&in)
			_, _ = res.Write(bytes)
		}
	}))
	callbacked := false
	rules := ""
	opt := &Options{
		API: testServer.URL,
		RulesUpdatedCallback: func(rulesString string, time int64) {
			rules = rulesString
			if time == 1631638014811 {
				callbacked = true
			}
		},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}

	InitializeWithOptions("secret-key", opt)

	if !callbacked {
		t.Errorf("rules updated callback did not happen")
	}

	if !CheckGate(User{UserID: "136"}, "fractional_gate") {
		t.Errorf("fractional_gate should return true for the given user")
	}

	ShutdownAndDangerouslyClearInstance()

	// Now use rules from the previous update callback to bootstrap, and validate values
	opt_bootstrap := &Options{
		BootstrapValues:      rules,
		LocalMode:            true,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opt_bootstrap)

	if !CheckGate(User{UserID: "123"}, "always_on_gate") {
		t.Errorf("always_on_gate should return true bootstrap value is provided")
	}

	ShutdownAndDangerouslyClearInstance()
}

func TestLogImmediate(t *testing.T) {
	env := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			t.Errorf("Expected ‘POST’ request, got '%s'", req.Method)
		}
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
			env = input.Events[0].User.StatsigEnvironment["tier"]
		}

		res.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	opt := &Options{
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opt)
	event := Event{EventName: "test_event", User: User{UserID: "123"}}
	response, err := LogImmediate([]Event{event})
	if response.StatusCode != http.StatusOK {
		t.Errorf("Status should be OK")
	}
	if err != nil {
		t.Errorf("Error should be nil")
	}
	if env != "test" {
		t.Errorf("Environment not set on user")
	}

	ShutdownAndDangerouslyClearInstance()
}

func TestVersion(t *testing.T) {
	metadata := getStatsigMetadata()
	versionsString, _ := exec.Command("go", "list", "-m", "-versions").Output()
	versions := strings.Fields(string(versionsString))
	currentVersion := versions[len(versions)-1]
	versionNumber := strings.Split(currentVersion, "v")[1]
	if metadata.SDKVersion != versionNumber {
		t.Errorf(
			"SDK version mismatch: %s (StatsigMetadata) %s (module)",
			metadata.SDKVersion, versionNumber,
		)
	}
}
