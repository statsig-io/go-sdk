package statsig

import (
	"testing"
)

func TestInitIDListWithSyncOnInitialize(t *testing.T) {
	var syncedIDListCount int

	testServer := getTestServer(testServerOptions{
		onGetIDLists: func() {
			syncedIDListCount += 1
		},
	})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  true,
		},
		DisableIdList: false,
	}

	InitializeWithOptions("secret-key", options)
	if syncedIDListCount <= 0 {
		t.Errorf("Expected 1 call to get id list but got %d", syncedIDListCount)
	}
	ShutdownAndDangerouslyClearInstance()

}

func TestInitIDListWithoutSyncOnInitialize(t *testing.T) {
	var syncedIDListCount int

	testServer := getTestServer(testServerOptions{
		onGetIDLists: func() {
			syncedIDListCount += 1
		},
	})
	defer testServer.Close()

	options := &Options{
		API:                 testServer.URL,
		Environment:         Environment{Tier: "test"},
		OutputLoggerOptions: getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: false,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  true,
		},
		DisableIdList: true,
	}

	InitializeWithOptions("secret-key", options)
	if syncedIDListCount > 0 {
		t.Errorf("Expected 0 calls to get id list but got %d", syncedIDListCount)
	}

	ShutdownAndDangerouslyClearInstance()

}
