package statsig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeTestServer(reqCallback func(req *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		reqCallback(req)
	}))
}

func TestStatsigMetadata(t *testing.T) {
	sessionID := ""
	InitializeWithOptions("secret-key", &Options{
		API: makeTestServer(func(req *http.Request) {
			reqSessionID := req.Header.Get("STATSIG-SERVER-SESSION-ID")
			if strings.Contains(req.URL.Path, "download_config_specs") {
				sessionID = reqSessionID
			}
			if strings.Contains(req.URL.Path, "log_event") {
				if reqSessionID != sessionID {
					t.Error("Inconsistent SessionID")
				}
			}
		}).URL,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
		LoggingMaxBufferSize: 1,
	})
	if sessionID == "" {
		t.Error("Missing SessionID in statsig metadata")
	}
	CheckGate(User{UserID: "first"}, "non-existent")
	Shutdown()
	InitializeWithOptions("secret-key", &Options{
		API: makeTestServer(func(req *http.Request) {
			reqSessionID := req.Header.Get("STATSIG-SERVER-SESSION-ID")
			if strings.Contains(req.URL.Path, "download_config_specs") {
				if reqSessionID == sessionID {
					t.Error("SessionID not reset on Initialize")
				}
			}
		}).URL,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(),
	})
	ShutdownAndDangerouslyClearInstance()
}
