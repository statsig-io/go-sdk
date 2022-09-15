package statsig

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBootstrap(t *testing.T) {
	bytes, _ := ioutil.ReadFile("download_config_specs.json")
	Initialize("secret-key")
	if CheckGate(User{UserID: "123"}, "always_on_gate") {
		t.Errorf("always_on_gate should return false when there is no bootstrap value")
	}
	shutDownAndClearInstance()

	opt := &Options{
		BootstrapValues: string(bytes[:]),
	}
	InitializeWithOptions("secret-key", opt)

	if !CheckGate(User{UserID: "123"}, "always_on_gate") {
		t.Errorf("always_on_gate should return true bootstrap value is provided")
	}
	shutDownAndClearInstance()
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
		API:         testServer.URL,
		Environment: Environment{Tier: "test"},
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

	shutDownAndClearInstance()
}
