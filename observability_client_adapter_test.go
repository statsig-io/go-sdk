package statsig

import (
	"context"
	"testing"
)

type testRecordedCall struct {
	MetricName string
	Value      float64
	Tags       map[string]interface{}
}

type testObservabilityClient struct {
	BaseObservabilityClient
	initCalled bool
	incCall    *testRecordedCall
	gaugeCall  *testRecordedCall
	distCall   *testRecordedCall
}

func (m *testObservabilityClient) Init(ctx context.Context) error {
	m.initCalled = true
	return nil
}

func (m *testObservabilityClient) Increment(metricName string, value int, tags map[string]interface{}) error {
	m.incCall = &testRecordedCall{
		MetricName: metricName,
		Value:      float64(value),
		Tags:       tags,
	}
	return nil
}

func (m *testObservabilityClient) Gauge(metricName string, value float64, tags map[string]interface{}) error {
	m.gaugeCall = &testRecordedCall{
		MetricName: metricName,
		Value:      value,
		Tags:       tags,
	}
	return nil
}

func (m *testObservabilityClient) Distribution(metricName string, value float64, tags map[string]interface{}) error {
	m.distCall = &testRecordedCall{
		MetricName: metricName,
		Value:      value,
		Tags:       tags,
	}
	return nil
}

func TestNewObservabilityClientFromHandlerBindsMethods(t *testing.T) {
	mock := &testObservabilityClient{}
	client := NewObservabilityClientFromHandler(mock)

	_ = client.Init(context.Background())
	_ = client.Increment("test_inc", 123, map[string]interface{}{
		"test_tag": "inc_test_value",
	})
	_ = client.Gauge("test_gauge", 111, map[string]interface{}{
		"test_tag": "gauge_test_value",
	})
	_ = client.Distribution("test_dist", 88, map[string]interface{}{
		"test_tag": "dist_test_value",
	})

	if !mock.initCalled {
		t.Error("expected init callback to be invoked")
	}
	if mock.incCall == nil || mock.incCall.MetricName != "test_inc" || mock.incCall.Value != 123 {
		t.Error("expected increment callback to be invoked with metric/value")
	}
	if mock.gaugeCall == nil || mock.gaugeCall.MetricName != "test_gauge" || mock.gaugeCall.Value != 111 {
		t.Error("expected gauge callback to be invoked with metric/value")
	}
	if mock.distCall == nil || mock.distCall.MetricName != "test_dist" || mock.distCall.Value != 88 {
		t.Error("expected distribution callback to be invoked with metric/value")
	}

	if mock.incCall.Tags["sdk_type"] != goSDKTypeTagValue {
		t.Error("expected increment tags to include sdk_type")
	}
	if mock.gaugeCall.Tags["sdk_type"] != goSDKTypeTagValue {
		t.Error("expected gauge tags to include sdk_type")
	}
	if mock.distCall.Tags["sdk_type"] != goSDKTypeTagValue {
		t.Error("expected distribution tags to include sdk_type")
	}
}

func TestNewObservabilityClientDefaultsAreNoop(t *testing.T) {
	client := NewObservabilityClient(ObservabilityClientFunctions{})

	if err := client.Init(context.Background()); err != nil {
		t.Errorf("expected no-op init to succeed: %v", err)
	}
	if err := client.Increment("test_inc", 1, nil); err != nil {
		t.Errorf("expected no-op increment to succeed: %v", err)
	}
	if err := client.Gauge("test_gauge", 1, nil); err != nil {
		t.Errorf("expected no-op gauge to succeed: %v", err)
	}
	if err := client.Distribution("test_dist", 1, nil); err != nil {
		t.Errorf("expected no-op distribution to succeed: %v", err)
	}
	if err := client.Shutdown(context.Background()); err != nil {
		t.Errorf("expected no-op shutdown to succeed: %v", err)
	}
}

func TestNewObservabilityClientFromNilHandlerUsesNoopDefaults(t *testing.T) {
	client := NewObservabilityClientFromHandler(nil)

	if err := client.Init(context.Background()); err != nil {
		t.Errorf("expected no-op init to succeed: %v", err)
	}
	if err := client.Increment("test_inc", 1, map[string]interface{}{}); err != nil {
		t.Errorf("expected no-op increment to succeed: %v", err)
	}
}

func TestInitializeGlobalOutputLoggerWrapsProvidedObservabilityClientWithSDKTypeTag(t *testing.T) {
	mock := &testObservabilityClient{}
	InitializeGlobalOutputLogger(OutputLoggerOptions{}, mock)

	Logger().Increment("test_metric", 1, map[string]interface{}{
		"source": "test",
	})

	if mock.incCall == nil {
		t.Fatal("expected increment callback to be invoked")
	}
	if mock.incCall.Tags["source"] != "test" {
		t.Error("expected existing tags to be preserved")
	}
	if mock.incCall.Tags["sdk_type"] != goSDKTypeTagValue {
		t.Error("expected sdk_type tag to be injected")
	}
}
