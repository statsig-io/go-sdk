package types

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
	if v, ok := d.Value[key]; ok {
		var res string
		switch v.(type) {
		case string:
			res = v.(string)
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
		switch v.(type) {
		case float64:
			res = v.(float64)
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
		switch v.(type) {
		case bool:
			res = v.(bool)
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
		switch v.(type) {
		case []interface{}:
			res = v.([]interface{})
		default:
			res = fallback
		}
		return res
	}
	return fallback
}

