package statsig

import (
	"context"
	"errors"
	"sync"
)

type Metric struct {
	Name  string                 `json:"name"`
	Type  string                 `json:"type"`
	Value float64                `json:"value"`
	Tags  map[string]interface{} `json:"tags"`
}

type observabilityClientExample struct {
	incrementMetrics    []Metric
	gaugeMetrics        []Metric
	distributionMetrics []Metric
	mu                  sync.RWMutex
}

func NewObservabilityClientExample() *observabilityClientExample {
	return &observabilityClientExample{
		incrementMetrics:    make([]Metric, 0),
		gaugeMetrics:        make([]Metric, 0),
		distributionMetrics: make([]Metric, 0),
	}
}

func (o *observabilityClientExample) Init(ctx context.Context) error {
	return nil
}

func (o *observabilityClientExample) Increment(metricName string, value int, tags map[string]interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.incrementMetrics = append(o.incrementMetrics, Metric{
		Name:  metricName,
		Type:  "increment",
		Value: float64(value),
		Tags:  tags,
	})
	return nil
}

func (o *observabilityClientExample) Gauge(metricName string, value float64, tags map[string]interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.gaugeMetrics = append(o.gaugeMetrics, Metric{
		Name:  metricName,
		Type:  "gauge",
		Value: value,
		Tags:  tags,
	})
	return nil
}

func (o *observabilityClientExample) Distribution(metricName string, value float64, tags map[string]interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.distributionMetrics = append(o.distributionMetrics, Metric{
		Name:  metricName,
		Type:  "distribution",
		Value: value,
		Tags:  tags,
	})
	return nil
}

func (o *observabilityClientExample) ShouldEnableHighCardinalityForThisTag(tag string) bool {
	return true
}

func (o *observabilityClientExample) Shutdown(ctx context.Context) error {
	return nil
}

func (o *observabilityClientExample) GetMetrics(metricType string) []Metric {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if metricType == "" {
		totalLength := len(o.incrementMetrics) + len(o.gaugeMetrics) + len(o.distributionMetrics)
		metrics := make([]Metric, 0, totalLength)
		metrics = append(metrics, o.incrementMetrics...)
		metrics = append(metrics, o.gaugeMetrics...)
		metrics = append(metrics, o.distributionMetrics...)

		return metrics
	}

	switch metricType {
	case "increment":
		metrics := make([]Metric, len(o.incrementMetrics))
		copy(metrics, o.incrementMetrics)
		return metrics
	case "gauge":
		metrics := make([]Metric, len(o.gaugeMetrics))
		copy(metrics, o.gaugeMetrics)
		return metrics
	case "distribution":
		metrics := make([]Metric, len(o.distributionMetrics))
		copy(metrics, o.distributionMetrics)
		return metrics
	default:
		return []Metric{}
	}
}

func (o *observabilityClientExample) ClearMetrics() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.incrementMetrics = make([]Metric, 0)
	o.gaugeMetrics = make([]Metric, 0)
	o.distributionMetrics = make([]Metric, 0)
}

type brokenObservabilityClientExample struct{}

func NewBrokenObservabilityClientExample() *brokenObservabilityClientExample {
	return &brokenObservabilityClientExample{}
}

func (o *brokenObservabilityClientExample) Init(ctx context.Context) error {
	return errors.New("init failed")
}

func (o *brokenObservabilityClientExample) Increment(metricName string, value int, tags map[string]interface{}) error {
	return errors.New("increment failed")
}

func (o *brokenObservabilityClientExample) Gauge(metricName string, value float64, tags map[string]interface{}) error {
	return errors.New("gauge failed")
}

func (o *brokenObservabilityClientExample) Distribution(metricName string, value float64, tags map[string]interface{}) error {
	return errors.New("distribution failed")
}

func (o *brokenObservabilityClientExample) ShouldEnableHighCardinalityForThisTag(tag string) bool {
	return false
}

func (o *brokenObservabilityClientExample) Shutdown(ctx context.Context) error {
	return errors.New("shutdown failed")
}
