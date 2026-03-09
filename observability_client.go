package statsig

import (
	"context"
)

const goSDKTypeTagValue = "statsig-server-go"

type ObservabilityClientFunctions struct {
	Init                                  func(ctx context.Context) error
	Increment                             func(metricName string, value int, tags map[string]interface{}) error
	Gauge                                 func(metricName string, value float64, tags map[string]interface{}) error
	Distribution                          func(metricName string, value float64, tags map[string]interface{}) error
	ShouldEnableHighCardinalityForThisTag func(tag string) bool
	Shutdown                              func(ctx context.Context) error
}

// ObservabilityClientHandler mirrors the server-core construction pattern
// while keeping legacy-go method signatures.
type ObservabilityClientHandler interface {
	Init(ctx context.Context) error
	Increment(metricName string, value int, tags map[string]interface{}) error
	Gauge(metricName string, value float64, tags map[string]interface{}) error
	Distribution(metricName string, value float64, tags map[string]interface{}) error
	ShouldEnableHighCardinalityForThisTag(tag string) bool
	Shutdown(ctx context.Context) error
}

// BaseObservabilityClient provides no-op defaults so callers can override
// only the callbacks they need.
type BaseObservabilityClient struct{}

func (BaseObservabilityClient) Init(context.Context) error { return nil }

func (BaseObservabilityClient) Increment(string, int, map[string]interface{}) error { return nil }

func (BaseObservabilityClient) Gauge(string, float64, map[string]interface{}) error { return nil }

func (BaseObservabilityClient) Distribution(string, float64, map[string]interface{}) error { return nil }

func (BaseObservabilityClient) ShouldEnableHighCardinalityForThisTag(string) bool { return false }

func (BaseObservabilityClient) Shutdown(context.Context) error { return nil }

type ObservabilityClient struct {
	functions ObservabilityClientFunctions
}

func withSDKTypeTag(tags map[string]interface{}) map[string]interface{} {
	if tags == nil {
		return map[string]interface{}{
			"sdk_type": goSDKTypeTagValue,
		}
	}

	cloned := make(map[string]interface{}, len(tags)+1)
	for k, v := range tags {
		cloned[k] = v
	}

	if _, exists := cloned["sdk_type"]; !exists {
		cloned["sdk_type"] = goSDKTypeTagValue
	}

	return cloned
}

func withDefaultObservabilityClientFunctions(functions ObservabilityClientFunctions) ObservabilityClientFunctions {
	if functions.Init == nil {
		functions.Init = func(context.Context) error { return nil }
	}
	if functions.Increment == nil {
		functions.Increment = func(string, int, map[string]interface{}) error { return nil }
	}
	if functions.Gauge == nil {
		functions.Gauge = func(string, float64, map[string]interface{}) error { return nil }
	}
	if functions.Distribution == nil {
		functions.Distribution = func(string, float64, map[string]interface{}) error { return nil }
	}
	if functions.ShouldEnableHighCardinalityForThisTag == nil {
		functions.ShouldEnableHighCardinalityForThisTag = func(string) bool { return false }
	}
	if functions.Shutdown == nil {
		functions.Shutdown = func(context.Context) error { return nil }
	}

	return functions
}

// NewObservabilityClientFromHandler binds interface methods into a concrete
// IObservabilityClient with defaults for unimplemented behavior via embedding
// BaseObservabilityClient.
func NewObservabilityClientFromHandler(handler ObservabilityClientHandler) *ObservabilityClient {
	if handler == nil {
		handler = BaseObservabilityClient{}
	}

	return NewObservabilityClient(ObservabilityClientFunctions{
		Init:                                  handler.Init,
		Increment:                             handler.Increment,
		Gauge:                                 handler.Gauge,
		Distribution:                          handler.Distribution,
		ShouldEnableHighCardinalityForThisTag: handler.ShouldEnableHighCardinalityForThisTag,
		Shutdown:                              handler.Shutdown,
	})
}

func NewObservabilityClient(functions ObservabilityClientFunctions) *ObservabilityClient {
	return &ObservabilityClient{
		functions: withDefaultObservabilityClientFunctions(functions),
	}
}

func (c *ObservabilityClient) Init(ctx context.Context) error {
	return c.functions.Init(ctx)
}

func (c *ObservabilityClient) Increment(metricName string, value int, tags map[string]interface{}) error {
	return c.functions.Increment(metricName, value, withSDKTypeTag(tags))
}

func (c *ObservabilityClient) Gauge(metricName string, value float64, tags map[string]interface{}) error {
	return c.functions.Gauge(metricName, value, withSDKTypeTag(tags))
}

func (c *ObservabilityClient) Distribution(metricName string, value float64, tags map[string]interface{}) error {
	return c.functions.Distribution(metricName, value, withSDKTypeTag(tags))
}

func (c *ObservabilityClient) ShouldEnableHighCardinalityForThisTag(tag string) bool {
	return c.functions.ShouldEnableHighCardinalityForThisTag(tag)
}

func (c *ObservabilityClient) Shutdown(ctx context.Context) error {
	return c.functions.Shutdown(ctx)
}

type taggedObservabilityClient struct {
	inner IObservabilityClient
}

func wrapObservabilityClientWithTags(client IObservabilityClient) IObservabilityClient {
	if client == nil {
		return nil
	}
	return &taggedObservabilityClient{inner: client}
}

func (c *taggedObservabilityClient) Init(ctx context.Context) error {
	return c.inner.Init(ctx)
}

func (c *taggedObservabilityClient) Increment(metricName string, value int, tags map[string]interface{}) error {
	return c.inner.Increment(metricName, value, withSDKTypeTag(tags))
}

func (c *taggedObservabilityClient) Gauge(metricName string, value float64, tags map[string]interface{}) error {
	return c.inner.Gauge(metricName, value, withSDKTypeTag(tags))
}

func (c *taggedObservabilityClient) Distribution(metricName string, value float64, tags map[string]interface{}) error {
	return c.inner.Distribution(metricName, value, withSDKTypeTag(tags))
}

func (c *taggedObservabilityClient) ShouldEnableHighCardinalityForThisTag(tag string) bool {
	return c.inner.ShouldEnableHighCardinalityForThisTag(tag)
}

func (c *taggedObservabilityClient) Shutdown(ctx context.Context) error {
	return c.inner.Shutdown(ctx)
}
