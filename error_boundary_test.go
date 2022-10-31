package statsig

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mock_server(t *testing.T, expectedError error, hit *bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		*hit = true
		var body *logExceptionRequestBody
		_ = json.NewDecoder(req.Body).Decode(&body)
		if expectedError == nil || body.Exception == expectedError.Error() {
			success := &logExceptionResponse{Success: true}
			json, _ := json.Marshal(success)
			_, _ = res.Write(json)
		} else {
			t.Error("Failed to log exception")
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
	errorBoundary := newErrorBoundary(opt)
	errorBoundary.logException(err)
	if !hit {
		t.Error("Expected logException to be hit")
	}
}

func TestInvalidSDKKeyError(t *testing.T) {
	expectedError := errors.New(InvalidSDKKeyError)
	hit := false
	testServer := mock_server(t, expectedError, &hit)
	defer testServer.Close()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected function to panic")
		}
	}()
	opt := &Options{
		API:         testServer.URL,
		Environment: Environment{Tier: "test"},
	}
	InitializeWithOptions("invalid-sdk-key", opt)
	if !hit {
		t.Error("Expected logException to be hit")
	}
	shutDownAndClearInstance()
}
