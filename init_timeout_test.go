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
			time.Sleep(100 * time.Millisecond)
		}
		res.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	user := User{UserID: "some_user_id"}
	initTimeBuffer := 2 * time.Millisecond // expected runtime buffer for initialize with timeout

	t.Run("No timeout option", func(t *testing.T) {
		options := &Options{
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if elapsed < (100 * time.Millisecond) {
			t.Errorf("Expected initalize to take at least 1 second")
		}
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("Expected initialize to succeed")
			}
		}()
		CheckGate(user, "nonexistent-gate")
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Initalize finish before timeout", func(t *testing.T) {
		options := &Options{
			API:                  testServer.URL,
			InitTimeout:          5 * time.Second,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if elapsed < (100 * time.Millisecond) {
			t.Errorf("Expected initalize to take at least 1 second")
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
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Initialize timed out", func(t *testing.T) {
		options := &Options{
			API:                  testServer.URL,
			InitTimeout:          100 * time.Millisecond,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if elapsed > (options.InitTimeout + initTimeBuffer) {
			t.Errorf("Initalize exceeded timeout %s", elapsed)
		}
		gate := CheckGate(user, "always_on_gate")
		if gate != false {
			t.Errorf("Expected gate to be default off")
		}
		ShutdownAndDangerouslyClearInstance()
	})
}
