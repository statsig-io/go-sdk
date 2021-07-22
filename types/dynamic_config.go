package types

// A json blob configured in the Statsig Console
type DynamicConfig struct {
	Name   string                 `json:"name"`
	Value  map[string]interface{} `json:"value"`
	RuleID string                 `json:"rule_id"`
}

func NewConfig(name string, value map[string]interface{}, ruleID string) *DynamicConfig {
	if value == nil {
		value = make(map[string]interface{})
	}
	return &DynamicConfig{
		Name:   name,
		Value:  value,
		RuleID: ruleID,
	}
}

// Gets the string value at the given key in the DynamicConfig
// Returns the fallback string if the item at the given key is not found or not of type string
func (d *DynamicConfig) GetString(key string, fallback string) string {
	if v, ok := d.Value[key]; ok {
		var res string
		switch val := v.(type) {
		case string:
			res = val
		default:
			res = fallback
		}
		return res
	}
	return fallback
}

// Gets the float64 value at the given key in the DynamicConfig
// Returns the fallback float64 if the item at the given key is not found or not of type float64
func (d *DynamicConfig) GetNumber(key string, fallback float64) float64 {
	if v, ok := d.Value[key]; ok {
		var res float64
		switch val := v.(type) {
		case float64:
			res = val
		default:
			res = fallback
		}
		return res
	}
	return fallback
}

// Gets the boolean value at the given key in the DynamicConfig
// Returns the fallback boolean if the item at the given key is not found or not of type boolean
func (d *DynamicConfig) GetBool(key string, fallback bool) bool {
	if v, ok := d.Value[key]; ok {
		var res bool
		switch val := v.(type) {
		case bool:
			res = val
		default:
			res = fallback
		}
		return res
	}
	return fallback
}

// Gets the slice value at the given key in the DynamicConfig
// Returns the fallback slice if the item at the given key is not found or not of type slice
func (d *DynamicConfig) GetSlice(key string, fallback []interface{}) []interface{} {
	if v, ok := d.Value[key]; ok {
		var res = make([]interface{}, 0)
		switch val := v.(type) {
		case []interface{}:
			res = val
		default:
			res = fallback
		}
		return res
	}
	return fallback
}
