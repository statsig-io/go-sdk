//go:build !race
// +build !race

package statsig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStoreSyncFailure(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/download_config_specs") {
			res.WriteHeader(500)
			return
		}
	}))
	defer testServer.Close()
	opt := &Options{
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		ConfigSyncInterval:   100 * time.Millisecond,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
	}

	syncOutdatedMax = 200 * time.Millisecond
	stderrLogs := swallowStderr(func() {
		InitializeWithOptions("secret-key", opt)
	})
	if stderrLogs == "" {
		t.Error("Expected error message in stderr")
	}
	stderrLogs = swallowStderr(func() {
		time.Sleep(100 * time.Millisecond)
	})
	if stderrLogs != "" {
		t.Error("Expected no output to stderr")
	}
	stderrLogs = swallowStderr(func() {
		time.Sleep(100 * time.Millisecond)
	})
	if stderrLogs == "" {
		t.Error("Expected error message in stderr")
	}
}
