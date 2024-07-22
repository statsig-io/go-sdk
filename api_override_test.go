package statsig

import (
	"net/http/httptest"
	"testing"
)

type counters struct {
	dcs        int
	getIDLists int
	logEvent   int
}

func setupTestServer(counts *counters) *httptest.Server {
	return getTestServer(testServerOptions{
		dcsOnline: true,
		onLogEvent: func(events []map[string]interface{}) {
			counts.logEvent += 1
		},
		onDCS: func() {
			counts.dcs += 1
		},
		onGetIDLists: func() {
			counts.getIDLists += 1
		},
	})
}

func TestAPIOverride(t *testing.T) {
	counts := &counters{dcs: 0, getIDLists: 0, logEvent: 0}
	testServer := setupTestServer(counts)
	opts := &Options{
		API:                  testServer.URL,
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opts)
	user := User{UserID: "some_user_id"}
	CheckGate(user, "test_gate")
	ShutdownAndDangerouslyClearInstance()

	if counts.dcs < 1 {
		t.Error("Expected call to download_config_specs")
	}
	if counts.getIDLists < 1 {
		t.Error("Expected call to get_id_lists")
	}
	if counts.logEvent < 1 {
		t.Error("Expected call to log_event")
	}
}

func TestAPIOverrideDCS(t *testing.T) {
	counts := &counters{dcs: 0, getIDLists: 0, logEvent: 0}
	testServer := setupTestServer(counts)
	opts := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: testServer.URL,
		},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opts)
	user := User{UserID: "some_user_id"}
	CheckGate(user, "test_gate")
	ShutdownAndDangerouslyClearInstance()

	if counts.dcs < 1 {
		t.Error("Expected call to download_config_specs")
	}
	if counts.getIDLists != 0 {
		t.Error("Expected zero calls to get_id_lists")
	}
	if counts.logEvent != 0 {
		t.Error("Expected zero calls to log_event")
	}
}

func TestAPIOverrideGetIDLists(t *testing.T) {
	counts := &counters{dcs: 0, getIDLists: 0, logEvent: 0}
	testServer := setupTestServer(counts)
	opts := &Options{
		APIOverrides: APIOverrides{
			GetIDLists: testServer.URL,
		},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opts)
	user := User{UserID: "some_user_id"}
	CheckGate(user, "test_gate")
	ShutdownAndDangerouslyClearInstance()

	if counts.dcs != 0 {
		t.Error("Expected zero calls to download_config_specs")
	}
	if counts.getIDLists < 1 {
		t.Error("Expected call to get_id_lists")
	}
	if counts.logEvent != 0 {
		t.Error("Expected zero calls to log_event")
	}
}

func TestAPIOverrideLogEvent(t *testing.T) {
	counts := &counters{dcs: 0, getIDLists: 0, logEvent: 0}
	testServer := setupTestServer(counts)
	opts := &Options{
		APIOverrides: APIOverrides{
			LogEvent: testServer.URL,
		},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}
	InitializeWithOptions("secret-key", opts)
	user := User{UserID: "some_user_id"}
	CheckGate(user, "test_gate")
	ShutdownAndDangerouslyClearInstance()

	if counts.dcs != 0 {
		t.Error("Expected zero calls to download_config_specs")
	}
	if counts.getIDLists != 0 {
		t.Error("Expected zero call to get_id_lists")
	}
	if counts.logEvent < 1 {
		t.Error("Expected call to log_event")
	}
}
