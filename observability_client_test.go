package statsig

import (
	"os"
	"testing"
	"time"
)

func TestObservabilityClientExample(t *testing.T) {
	t.Run("able to get initialization metrics", func(t *testing.T) {
		testServer := getTestServer(testServerOptions{})
		observabilityClient := NewObservabilityClientExample()
		options := &Options{
			ObservabilityClient: observabilityClient,
			API:                 testServer.URL,
		}
		observabilityClient.ClearMetrics()
		InitializeWithOptions("secret-key", options)
		defer ShutdownAndDangerouslyClearInstance()

		distributionMetrics := observabilityClient.GetMetrics("distribution")
		if len(distributionMetrics) == 0 {
			t.Errorf("Expected to receive distribution metrics for initialization")
		}

		initMetric := distributionMetrics[0]
		if initMetric.Name != "statsig.sdk.initialization" {
			t.Errorf("Expected metric to have name 'initialization'")
		}
		if initMetric.Value <= 0 {
			t.Errorf("Expected metric to have a value")
		}
		if initMetric.Tags == nil {
			t.Errorf("Expected metric to have tags")
		}
		if initMetric.Tags["source"] != string(SourceNetwork) {
			t.Errorf("Expected metric to have source tag")
		}
		if initMetric.Tags["init_source_api"] != testServer.URL {
			t.Errorf("Expected metric to have correct source_api tag")
		}
		if initMetric.Tags["store_populated"] != true {
			t.Errorf("Expected store_populated to be true")
		}
		if initMetric.Tags["init_success"] != true {
			t.Errorf("Expected init success to be true")
		}

	})

	t.Run("able to get config sync metrics", func(t *testing.T) {
		testServer := getTestServer(testServerOptions{
			useCurrentTime: true,
		})
		observabilityClient := NewObservabilityClientExample()
		options := &Options{
			ObservabilityClient: observabilityClient,
			API:                 testServer.URL,
			ConfigSyncInterval:  5 * time.Millisecond,
		}
		InitializeWithOptions("secret-key", options)
		defer ShutdownAndDangerouslyClearInstance()

		time.Sleep(10 * time.Millisecond)

		distributionMetrics := observabilityClient.GetMetrics("distribution")
		if len(distributionMetrics) == 0 {
			t.Errorf("Expected to receive distribution metrics for config sync")
		}
		configSyncMetric := distributionMetrics[1]
		if configSyncMetric.Name != "statsig.sdk.config_propagation_diff" {
			t.Errorf("Expected metric to have name 'config_propagation_diff'")
		}
		if configSyncMetric.Value <= 0 {
			t.Errorf("Expected metric to have a positive value")
		}
		if configSyncMetric.Tags["source"] != string(SourceNetwork) {
			t.Errorf("Expected metric to have source network tag")
		}
		if configSyncMetric.Tags["source_api"] != testServer.URL {
			t.Errorf("Expected metric to have correct source_api tag")
		}
		if configSyncMetric.Tags["lcut"] == 0 {
			t.Errorf("Expected metric to have lcut tag")
		}
		if configSyncMetric.Tags["prev_lcut"] == 0 {
			t.Errorf("Expected metric to have prev_lcut tag")
		}
	})

	t.Run("able to get config no update metrics", func(t *testing.T) {
		testServer := getTestServer(testServerOptions{
			noUpdateOnSync: true,
		})
		observabilityClient := NewObservabilityClientExample()
		options := &Options{
			ObservabilityClient: observabilityClient,
			API:                 testServer.URL,
			ConfigSyncInterval:  5 * time.Millisecond,
		}
		InitializeWithOptions("secret-key", options)
		defer ShutdownAndDangerouslyClearInstance()

		time.Sleep(10 * time.Millisecond)

		incrementMetrics := observabilityClient.GetMetrics("increment")
		if len(incrementMetrics) == 0 {
			t.Errorf("Expected to receive increment metrics for config no update")
		}
		configNoUpdateMetric := incrementMetrics[0]
		if configNoUpdateMetric.Name != "statsig.sdk.config_no_update" {
			t.Errorf("Expected metric to have name 'config_no_update'")
		}
		if configNoUpdateMetric.Tags["source"] != string(SourceNetwork) {
			t.Errorf("Expected metric to have source network tag")
		}
		if configNoUpdateMetric.Tags["source_api"] != testServer.URL {
			t.Errorf("Expected metric to have correct source_api tag")
		}
	})

}

func TestBrokenObservabilityClientExample(t *testing.T) {
	events := []Event{}
	testServer := getTestServer(testServerOptions{
		onLogEvent: func(newEvents []map[string]interface{}) {
			for _, newEvent := range newEvents {
				eventTyped := convertToExposureEvent(newEvent)
				events = append(events, eventTyped)
			}
		},
	})

	brokenObservabilityClient := NewBrokenObservabilityClientExample()
	options := &Options{
		ObservabilityClient:  brokenObservabilityClient,
		API:                  testServer.URL,
		Environment:          Environment{Tier: "test"},
		OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
	}

	t.Run("recover and continue when observability client throws errors", func(t *testing.T) {
		stderrLogs := swallow_stderr(func() {
			InitializeWithOptions("secret-key", options)
		})
		if stderrLogs == "" {
			t.Errorf("Expected output to stderr when observability client fails")
		}
		defer ShutdownAndDangerouslyClearInstance()
	})
}

func TestObservabilityClientWithBootstrap(t *testing.T) {
	dcs_bytes, _ := os.ReadFile("download_config_specs.json")
	testServer := getTestServer(testServerOptions{})

	dataAdapter := dataAdapterExample{store: make(map[string]string)}
	dataAdapter.Initialize()
	defer dataAdapter.Shutdown()
	dataAdapter.Set(CONFIG_SPECS_KEY, string(dcs_bytes))

	observabilityClient := NewObservabilityClientExample()
	options := &Options{
		DataAdapter:         &dataAdapter,
		ObservabilityClient: observabilityClient,
		API:                 testServer.URL,
	}

	t.Run("observability client works with data adapter", func(t *testing.T) {
		InitializeWithOptions("secret-key", options)
		defer ShutdownAndDangerouslyClearInstance()

		metrics := observabilityClient.GetMetrics("distribution")
		if len(metrics) == 0 {
			t.Errorf("Expected to receive metrics from observability client")
		}
		initMetric := metrics[0]
		if initMetric.Name != "statsig.sdk.initialization" {
			t.Errorf("Expected metric to have name 'initialization'")
		}
		if initMetric.Tags["source"] != string(SourceDataAdapter) {
			t.Errorf("Expected metric to have source tag as 'data_adapter'")
		}
		if initMetric.Tags["init_source_api"] != "" {
			t.Errorf("Expected metric to have empty init_source_api tag")
		}
		if initMetric.Tags["store_populated"] != true {
			t.Errorf("Expected store_populated to be true")
		}
	})
}
