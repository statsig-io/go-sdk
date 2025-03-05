package statsig

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func mock_server(t *testing.T, expectedError error, hit *bool, statsigOptions *map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/download_config_specs") {
			res.WriteHeader(500)
			return
		}
		if strings.Contains(req.URL.Path, "/sdk_exception") {
			var body *logExceptionRequestBody
			_ = json.NewDecoder(req.Body).Decode(&body)
			for key, value := range body.StatsigOptions {
				(*statsigOptions)[key] = value
			}
			if body.Exception != "" && (expectedError == nil || body.Exception == expectedError.Error()) {
				*hit = true
				success := &logExceptionResponse{Success: true}
				json, _ := json.Marshal(success)
				_, _ = res.Write(json)
			} else {
				t.Error("Failed to log exception")
			}
			return
		}
	}))
}

func TestLogException(t *testing.T) {
	err := errors.New("test error boundary log exception")
	hit := false
	statsigOption := make(map[string]interface{})
	testServer := mock_server(t, err, &hit, &statsigOption)
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	diagnostics := newDiagnostics(opt)
	errorBoundary := newErrorBoundary("client-key", opt, diagnostics)
	errorBoundary.logException(err)
	if !hit {
		t.Error("Expected sdk_exception endpoint to be hit")
	}
	if statsigOption["API"] != testServer.URL {
		t.Error("Expected statsigOptions to be set")
	}
}

func TestDCSError(t *testing.T) {
	hit := false
	statsigOption := make(map[string]interface{})
	testServer := mock_server(t, nil, &hit, &statsigOption)
	defer testServer.Close()
	dataadapter := dataAdapterExample{}
	opt := &Options{
		API:                  testServer.URL,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		DataAdapter:          &dataadapter,
	}
	InitializeWithOptions("secret-key", opt)
	defer ShutdownAndDangerouslyClearInstance()
	if !hit {
		t.Error("Expected sdk_exception endpoint to be hit")
	}
	expectedStatsigLoggerOptions := map[string]interface{}{
		"DisableInitDiagnostics": true,
		"DisableSyncDiagnostics": true,
		"DisableApiDiagnostics":  true,
		"DisableAllLogging":      false,
	}
	if !reflect.DeepEqual(statsigOption["StatsigLoggerOptions"], expectedStatsigLoggerOptions) {
		t.Error("Expected StatsigLoggerOptions in statsigOptions to be set")
	}
}

func TestRepeatedError(t *testing.T) {
	err := errors.New("common error")
	hit := false
	statsigOption := make(map[string]interface{})
	testServer := mock_server(t, err, &hit, &statsigOption)
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	diagnostics := newDiagnostics(opt)
	errorBoundary := newErrorBoundary("client-key", opt, diagnostics)
	errorBoundary.logException(err)
	if !hit {
		t.Error("Expected sdk_exception endpoint to be hit")
	}
	hit = false
	errorBoundary.logException(err)
	if hit {
		t.Error("Expected sdk_exception endpoint to NOT be hit")
	}
}

func TestStatsigOptions(t *testing.T) {
	hit := false
	statsigOption := make(map[string]interface{})
	testServer := mock_server(t, nil, &hit, &statsigOption)
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: testServer.URL,
		},
		DataAdapter:        &dataAdapterExample{},
		IDListSyncInterval: 1,
		IPCountryOptions: IPCountryOptions{
			Disabled: true,
			LazyLoad: true,
		},
	}
	diagnostics := newDiagnostics(opt)
	errorBoundary := newErrorBoundary("client-key", opt, diagnostics)
	errorBoundary.logException(errors.New("test error"))
	expectedOptions := map[string]interface{}{
		"API": testServer.URL,
		"APIOverrides": map[string]interface{}{
			"DownloadConfigSpecs": testServer.URL,
			"GetIDLists":          "",
			"LogEvent":            "",
		},
		"DataAdapter":        "set",
		"IDListSyncInterval": 1.0,
		"IPCountryOptions":   "set",
	}
	if !reflect.DeepEqual(statsigOption, expectedOptions) {
		t.Error("Expected statsigOptions to be set")
	}
}
