package statsig

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogException(t *testing.T) {
	err := errors.New("test error boundary log exception")
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			t.Errorf("Expected ‘POST’ request, got '%s'", req.Method)
		}
		var body *logExceptionRequestBody
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body.Exception == err.Error() {
			success := &logExceptionResponse{Success: true}
			json, _ := json.Marshal(success)
			res.Write(json)
		}
	}))
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	transport := newTransport("secret-123", opt)
	errorBoundary := newErrorBoundary(transport)
	success := errorBoundary.logException(err)
	if !success {
		t.Error("Failed to log exception")
	}
}
