package statsig

// User specific attributes for evaluating Feature Gates, Experiments, and DynamicConfigs
//
// NOTE: UserID is **required** - see https://docs.statsig.com/messages/serverRequiredUserID\
// PrivateAttributes are only used for user targeting/grouping in feature gates, dynamic configs,
// experiments and etc; they are omitted in logs.
type User struct {
	UserID             string                 `json:"userID"`
	Email              string                 `json:"email"`
	IpAddress          string                 `json:"ip"`
	UserAgent          string                 `json:"userAgent"`
	Country            string                 `json:"country"`
	Locale             string                 `json:"locale"`
	AppVersion         string                 `json:"appVersion"`
	Custom             map[string]interface{} `json:"custom"`
	PrivateAttributes  map[string]interface{} `json:"privateAttributes"`
	StatsigEnvironment map[string]string      `json:"statsigEnvironment"`
	CustomIDs          map[string]string      `json:"customIDs"`
}

// an event to be sent to Statsig for logging and analysis
type Event struct {
	EventName string            `json:"eventName"`
	User      User              `json:"user"`
	Value     string            `json:"value"`
	Metadata  map[string]string `json:"metadata"`
	Time      int64             `json:"time"`
}

type configBase struct {
	Name        string                 `json:"name"`
	Value       map[string]interface{} `json:"value"`
	RuleID      string                 `json:"rule_id"`
	GroupName   string                 `json:"group_name"`
	LogExposure *func(configBase, string)
}

// A json blob configured in the Statsig Console
type DynamicConfig struct {
	configBase
}

type Layer struct {
	configBase
}

func NewConfig(name string, value map[string]interface{}, ruleID string, groupName string) *DynamicConfig {
	if value == nil {
		value = make(map[string]interface{})
	}
	return &DynamicConfig{
		configBase{
			Name:      name,
			Value:     value,
			RuleID:    ruleID,
			GroupName: groupName,
		},
	}
}

func NewLayer(name string, value map[string]interface{}, ruleID string, groupName string, logExposure *func(configBase, string)) *Layer {
	if value == nil {
		value = make(map[string]interface{})
	}
	return &Layer{
		configBase{
			Name:        name,
			Value:       value,
			RuleID:      ruleID,
			GroupName:   groupName,
			LogExposure: logExposure,
		},
	}
}

// Gets the string value at the given key in the DynamicConfig
// Returns the fallback string if the item at the given key is not found or not of type string
func (d *configBase) GetString(key string, fallback string) string {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case string:
			logExposure(d, key)
			return val
		}
	}

	return fallback
}

// Gets the float64 value at the given key in the DynamicConfig
// Returns the fallback float64 if the item at the given key is not found or not of type float64
func (d *configBase) GetNumber(key string, fallback float64) float64 {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case float64:
			logExposure(d, key)
			return val
		}
	}
	return fallback
}

// Gets the boolean value at the given key in the DynamicConfig
// Returns the fallback boolean if the item at the given key is not found or not of type boolean
func (d *configBase) GetBool(key string, fallback bool) bool {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case bool:
			logExposure(d, key)
			return val
		}
	}
	return fallback
}

// Gets the slice value at the given key in the DynamicConfig
// Returns the fallback slice if the item at the given key is not found or not of type slice
func (d *configBase) GetSlice(key string, fallback []interface{}) []interface{} {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case []interface{}:
			logExposure(d, key)
			return val
		}
	}
	return fallback
}

func (d *configBase) GetMap(key string, fallback map[string]interface{}) map[string]interface{} {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case map[string]interface{}:
			logExposure(d, key)
			return val
		}
	}
	return fallback
}

func logExposure(c *configBase, parameterName string) {
	if c == nil || c.LogExposure == nil {
		return
	}

	l := *c.LogExposure
	l(*c, parameterName)
}
