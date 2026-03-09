package statsig

import (
	"os"
	"testing"
	"time"
)

func findMetricByName(metrics []Metric, metricName string) *Metric {
	for i := range metrics {
		if metrics[i].Name == metricName {
			return &metrics[i]
		}
	}
	return nil
}

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

		initMetric := findMetricByName(distributionMetrics, "statsig.sdk.initialization")
		if initMetric == nil {
			t.Fatalf("Expected to receive initialization metric")
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
		if initMetric.Tags["store_populated"] != "true" {
			t.Errorf("Expected store_populated to be 'true'")
		}
		if initMetric.Tags["init_success"] != "true" {
			t.Errorf("Expected init success to be 'true'")
		}
		if initMetric.Tags["sdk_key"] != "secret-key" {
			t.Errorf("Expected sdk_key to be loggable sdk key")
		}
		if initMetric.Tags["sdk_version"] == "" {
			t.Errorf("Expected sdk_version to be present")
		}
		if initMetric.Tags["sdk_type"] != "statsig-server-go" {
			t.Errorf("Expected sdk_type to be statsig-server-go")
		}
		networkLatencyMetricFound := false
		for _, metric := range distributionMetrics {
			if metric.Name == "statsig.sdk.network_request.latency" {
				networkLatencyMetricFound = true
				if metric.Tags["request_path"] == "" {
					t.Errorf("Expected network_request.latency request_path tag")
				}
				if metric.Tags["status_code"] == "" {
					t.Errorf("Expected network_request.latency status_code tag")
				}
				if metric.Tags["is_success"] == "" {
					t.Errorf("Expected network_request.latency is_success tag")
				}
				if metric.Tags["sdk_key"] == "" {
					t.Errorf("Expected network_request.latency sdk_key tag")
				}
				if metric.Tags["source_service"] == "" {
					t.Errorf("Expected network_request.latency source_service tag")
				}
				break
			}
		}
		if !networkLatencyMetricFound {
			t.Errorf("Expected to receive network_request.latency metric")
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
		configSyncMetric := findMetricByName(distributionMetrics, "statsig.sdk.config_propagation_diff")
		if configSyncMetric == nil {
			t.Fatalf("Expected metric to have name 'config_propagation_diff'")
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
		if _, ok := configSyncMetric.Tags["lcut"].(string); !ok {
			t.Errorf("Expected metric to have lcut tag")
		}
		if _, ok := configSyncMetric.Tags["prev_lcut"].(string); !ok {
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
		initMetric := findMetricByName(metrics, "statsig.sdk.initialization")
		if initMetric == nil {
			t.Fatalf("Expected metric to have name 'initialization'")
		}
		if initMetric.Tags["source"] != string(SourceDataAdapter) {
			t.Errorf("Expected metric to have source tag as 'data_adapter'")
		}
		if initMetric.Tags["init_source_api"] != "" {
			t.Errorf("Expected metric to have empty init_source_api tag")
		}
		if initMetric.Tags["store_populated"] != "true" {
			t.Errorf("Expected store_populated to be 'true'")
		}
	})
}
