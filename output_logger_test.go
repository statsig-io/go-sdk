package statsig

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLogEventErrors(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusBadRequest)
	}))
	defer testServer.Close()

	errs := make([]error, 0)
	opts := &Options{
		API: testServer.URL,
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  true,
		},
		OutputLoggerOptions: OutputLoggerOptions{
			EnableDebug: true,
			LogCallback: func(message string, err error) {
				errs = append(errs, err)
			},
		},
	}
	diagnostics := newDiagnostics(opts)
	transport := newTransport("secret", opts)
	errorBoundary := newErrorBoundary("secret", opts, nil)
	sdkConfigs := newSDKConfigs()
	logger := newLogger(transport, opts, diagnostics, errorBoundary, sdkConfigs)

	user := User{
		UserID: "123",
	}
	event := Event{
		EventName: "test_event",
		User:      user,
		Value:     "3",
	}

	stderrLogs := swallow_stderr(func() {
		logger.logCustom(event)
		logger.flush(true)
	})

	if stderrLogs == "" {
		t.Errorf("Expected output to stderr")
	}

	InitializeGlobalOutputLogger(opts.OutputLoggerOptions, nil)
	logger.logCustom(event)
	logger.flush(true)

	if len(errs) == 0 {
		t.Errorf("Expected output to callback")
	}

	if !errors.Is(errs[0], ErrFailedLogEvent) {
		t.Errorf("Expected error to match ErrFailedLogEvent")
	}

	if errs[0].Error() != "Failed to log 1 events: Failed request to /log_event after 0 retries: 400 Bad Request" {
		t.Errorf("Expected error message")
	}
}

func TestGetLoggableSDKKey(t *testing.T) {
	if got := getLoggableSDKKey("secret-1234567890abcdef"); got != "secret-123456" {
		t.Errorf("Expected long sdk key to be truncated, got %q", got)
	}

	if got := getLoggableSDKKey("secret-key"); got != "secret-key" {
		t.Errorf("Expected short sdk key to remain unchanged, got %q", got)
	}

	if got := getLoggableSDKKey(""); got != "" {
		t.Errorf("Expected empty sdk key to remain unchanged, got %q", got)
	}
}

func TestLogPostInitAddsNormalizedMetricTags(t *testing.T) {
	observabilityClient := NewObservabilityClientExample()
	InitializeGlobalOutputLogger(OutputLoggerOptions{}, observabilityClient)

	Logger().LogPostInit(&Options{}, "secret-1234567890abcdef", InitializeDetails{
		Duration:       1500 * time.Millisecond,
		Success:        true,
		Source:         SourceNetwork,
		SourceAPI:      "https://api.statsig.com/v1",
		StorePopulated: true,
	})

	distributionMetrics := observabilityClient.GetMetrics("distribution")
	if len(distributionMetrics) == 0 {
		t.Fatal("Expected initialization distribution metric to be emitted")
	}

	initMetric := distributionMetrics[len(distributionMetrics)-1]
	if initMetric.Name != "statsig.sdk.initialization" {
		t.Errorf("Expected initialization metric name, got %q", initMetric.Name)
	}
	if initMetric.Value != 1500 {
		t.Errorf("Expected initialization metric value in milliseconds, got %v", initMetric.Value)
	}
	if initMetric.Tags["init_success"] != "true" {
		t.Errorf("Expected init_success tag to be normalized to string true")
	}
	if initMetric.Tags["store_populated"] != "true" {
		t.Errorf("Expected store_populated tag to be normalized to string true")
	}
	if initMetric.Tags["sdk_key"] != "secret-123456" {
		t.Errorf("Expected sdk_key tag to be loggable prefix, got %v", initMetric.Tags["sdk_key"])
	}
	if initMetric.Tags["sdk_version"] == "" {
		t.Errorf("Expected sdk_version tag to be present")
	}
	if initMetric.Tags["sdk_type"] != goSDKTypeTagValue {
		t.Errorf("Expected sdk_type tag to be injected")
	}
}
