package statsig

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogImmediate(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			t.Errorf("Expected ‘POST’ request, got '%s'", req.Method)
		}
		res.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	InitializeWithOptions("secret-key", opt)
	response, err := LogImmediate(make([]Event, 1))
	if response.StatusCode != 200 {
		t.Errorf("Status should be 200")
	}
	if err != nil {
		t.Errorf("Error should be nil")
	}
}
