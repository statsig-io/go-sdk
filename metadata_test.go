package statsig

import (
	"sync"
	"testing"
)

func TestDerivedDeviceMetadata(t *testing.T) {
	var events events
	var mu sync.RWMutex

	testServer := getTestServer(testServerOptions{uaBasedRules: true,
		onLogEvent: func(newEvents []map[string]interface{}) {
			mu.Lock()
			events = append(events, newEvents...)
			mu.Unlock()
		}})
	defer testServer.Close()
	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  false,
		},
	}
	InitializeWithOptions("secret-key", options)

	browserNamePassUser := User{UserID: "browser_name_user_pass",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"}
	CheckGate(browserNamePassUser, "test_ua_browser_name")
	browserNameFailUser := User{UserID: "browser_name_user_fail",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Safari/537.3"}
	CheckGate(browserNameFailUser, "test_ua_browser_name")
	browserVersionPassUser := User{UserID: "browser_version_user_pass",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"}
	CheckGate(browserVersionPassUser, "test_ua_browser_version")
	browserVersionFailUser := User{UserID: "browser_version_user_fail",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/1.0 Safari/537.3"}
	CheckGate(browserVersionFailUser, "test_ua_browser_version")
	osNamePassUser := User{UserID: "os_name_user_pass",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"}
	CheckGate(osNamePassUser, "test_ua_os_name")
	osNameFailUser := User{UserID: "os_name_user_fail",
		UserAgent: "Mozilla/5.0 (iOS 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"}
	CheckGate(osNameFailUser, "test_ua_os_name")
	osVersionPassUser := User{UserID: "os_version_user_pass",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"}
	CheckGate(osVersionPassUser, "test_ua_os_version")
	osVersionFailUser := User{UserID: "os_version_user_fail",
		UserAgent: "Mozilla/5.0 (Windows NT 5.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"}
	CheckGate(osVersionFailUser, "test_ua_os_version")

	ShutdownAndDangerouslyClearInstance()

	if len(events) != 9 {
		t.Errorf("Expected 9 events, got %d", len(events))
	}

	metadata := getGateExposureEventMetadata(events)

	if len(metadata) != 8 {
		t.Errorf("Expected 8 gate exposure metadata, got %d", len(metadata))
	}

	if metadata[0]["browser_name"] != "Chrome" {
		t.Errorf("Expected gate to be Chrome, got %s", metadata[0]["browser_name"])
	}
	if metadata[1]["browser_name"] != "Safari" {
		t.Errorf("Expected gate to be Safari, got %s", metadata[1]["browser_name"])
	}
	if metadata[2]["browser_version"] != "58.0.3029" {
		t.Errorf("Expected gate to be 58.0.3029, got %s", metadata[2]["browser_version"])
	}
	if metadata[3]["browser_version"] != "1.0" {
		t.Errorf("Expected gate to be 1.0, got %s", metadata[3]["browser_version"])
	}
	if metadata[4]["os_name"] != "Windows" {
		t.Errorf("Expected gate to be Windows, got %s", metadata[4]["os_name"])
	}
	if metadata[5]["os_name"] != "iOS" {
		t.Errorf("Expected gate to be iOS, got %s", metadata[5]["os_name"])
	}
	if metadata[6]["os_version"] != "10" {
		t.Errorf("Expected gate to be 10, got %s", metadata[6]["os_version"])
	}
	if metadata[7]["os_version"] != "XP" {
		t.Errorf("Expected gate to be XP, got %s", metadata[7]["os_version"])
	}
}

func getGateExposureEventMetadata(events []map[string]interface{}) []map[string]interface{} {
	var metadataList []map[string]interface{}

	for _, event := range events {
		if event["eventName"] == "statsig::gate_exposure" {
			if metadata, ok := event["metadata"].(map[string]interface{}); ok {
				metadataList = append(metadataList, metadata)
			}
		}
	}

	return metadataList
}
