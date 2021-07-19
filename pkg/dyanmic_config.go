package statsig

import (
	"fmt"
)

type DynamicConfig struct {
	Name 	string
	Value   map[string]interface{}
	RuleID 	string
}

func NewConfig(name string, value map[string]interface{}, ruleID string) *DynamicConfig {
	return &DynamicConfig{
		Name: name,
		Value: value,
		RuleID: ruleID,
	}
}

func (d *DynamicConfig) GetString(key string, fallback string) string {
	v := d.Value[key]
	if v == nil {
		return fallback
	}
	var res string
	switch v.(type) {
	case string:
		res = v.(string)
	default:
		res = fallback
	}
	return res
}

func (d *DynamicConfig) GetNumber(key string, fallback float64) float64 {
	v := d.Value[key]
	if v == nil {
		return fallback
	}
	var res float64
	switch v.(type) {
	case float64:
		res = v.(float64)
	default:
		res = fallback
	}
	return res
}

func (d *DynamicConfig) GetBool(key string, fallback bool) bool {
	v := d.Value[key]
	if v == nil {
		return fallback
	}
	var res bool
	switch v.(type) {
	case bool:
		res = v.(bool)
	default:
		res = fallback
	}
	return res
}

func (d *DynamicConfig) GetSlice(key string, fallback ...interface{}) []interface{} {
	v := d.Value[key]
	if v == nil {
		return fallback
	}
	var res = make([]interface{}, 0)
	switch v.(type) {
	case []interface{}:
		res = v.([]interface{})
	default:
		res = fallback
	}
	fmt.Printf("%v\n", res)
	return res
}

