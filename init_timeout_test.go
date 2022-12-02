package statsig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInitTimeout(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			time.Sleep(2 * time.Second)
		}
		res.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	user := User{UserID: "some_user_id"}
	initTimeBuffer := 2 * time.Millisecond // expected runtime buffer for initialize with timeout

	t.Run("No timeout option", func(t *testing.T) {
		options := &Options{
			API: testServer.URL,
		}
		start := time.Now()
		InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if elapsed < (2 * time.Second) {
			t.Errorf("Expected initalize to take at least 2 seconds")
		}
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("Expected initialize to succeed")
			}
		}()
		CheckGate(user, "nonexistent-gate")
		shutDownAndClearInstance()
	})

	t.Run("Initalize finish before timeout", func(t *testing.T) {
		options := &Options{
			API:         testServer.URL,
			InitTimeout: 5 * time.Second,
		}
		start := time.Now()
		InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if elapsed < (2 * time.Second) {
			t.Errorf("Expected initalize to take at least 2 seconds")
		}
		if elapsed > (options.InitTimeout + initTimeBuffer) {
			t.Errorf("Initalize exceeded timeout")
		}
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("Expected initialize to succeed")
			}
		}()
		CheckGate(user, "nonexistent-gate")
		shutDownAndClearInstance()
	})

	t.Run("Initialize timed out", func(t *testing.T) {
		options := &Options{
			API:         testServer.URL,
			InitTimeout: 1 * time.Second,
		}
		start := time.Now()
		InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if elapsed > (options.InitTimeout + initTimeBuffer) {
			t.Errorf("Initalize exceeded timeout %s", elapsed)
		}
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("Expected initialize to fail")
			}
		}()
		CheckGate(user, "nonexistent-gate")
		shutDownAndClearInstance()
	})
}
