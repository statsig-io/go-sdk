package types

type DynamicConfig struct {
	Name   string
	Value  map[string]interface{}
	RuleID string
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
