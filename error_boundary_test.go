package statsig

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mock_server(t *testing.T, expectedError error, hit *bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/download_config_specs") {
			res.WriteHeader(500)
			return
		}
		if strings.Contains(req.URL.Path, "/sdk_exception") {
			var body *logExceptionRequestBody
			_ = json.NewDecoder(req.Body).Decode(&body)
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
	testServer := mock_server(t, err, &hit)
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
}

func TestDCSError(t *testing.T) {
	hit := false
	testServer := mock_server(t, nil, &hit)
	defer testServer.Close()
	opt := &Options{
		API:                  testServer.URL,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opt)
	defer ShutdownAndDangerouslyClearInstance()
	if !hit {
		t.Error("Expected sdk_exception endpoint to be hit")
	}
}

func TestRepeatedError(t *testing.T) {
	err := errors.New("common error")
	hit := false
	testServer := mock_server(t, err, &hit)
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
