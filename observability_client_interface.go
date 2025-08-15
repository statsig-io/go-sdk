package statsig

import (
	"context"
)

/**
 * IObservabilityClient is an interface for observability clients that allows users to plug in their
 * own observability integration for metrics collection and monitoring.
 */
type IObservabilityClient interface {
	/**
	 * Init initializes the observability client with necessary configuration.
	 * The context parameter allows for cancellation and timeout control.
	 */
	Init(ctx context.Context) error

	/**
	 * Increment increments a counter metric.
	 * metricName: The name of the metric to increment.
	 * value: The value by which the counter should be incremented (default is 1).
	 * tags: Optional map of tags for metric dimensions.
	 */
	Increment(metricName string, value int, tags map[string]interface{}) error

	/**
	 * Gauge sets a gauge metric.
	 * metricName: The name of the metric to set.
	 * value: The value to set the gauge to.
	 * tags: Optional map of tags for metric dimensions.
	 */
	Gauge(metricName string, value float64, tags map[string]interface{}) error

	/**
	 * Distribution records a distribution metric for tracking statistical data.
	 * metricName: The name of the metric to record.
	 * value: The recorded value for the distribution metric.
	 * tags: Optional map of tags that represent dimensions to associate with the metric.
	 */
	Distribution(metricName string, value float64, tags map[string]interface{}) error

	/**
	 * ShouldEnableHighCardinalityForThisTag determines if a high cardinality tag should be logged.
	 * tag: The tag to check for high cardinality enabled.
	 */
	ShouldEnableHighCardinalityForThisTag(tag string) bool

	/**
	 * Shutdown shuts down the observability client.
	 */
	Shutdown(ctx context.Context) error
}
