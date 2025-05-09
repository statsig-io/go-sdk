package statsig

// User specific attributes for evaluating Feature Gates, Experiments, and DynamicConfigs
//
// NOTE: UserID is **required** - see https://docs.statsig.com/messages/serverRequiredUserID\
// PrivateAttributes are only used for user targeting/grouping in feature gates, dynamic configs,
// experiments and etc; they are omitted in logs.
type User struct {
	UserID             string                 `json:"userID"`
	Email              string                 `json:"email,omitempty"`
	IpAddress          string                 `json:"ip,omitempty"`
	UserAgent          string                 `json:"userAgent,omitempty"`
	Country            string                 `json:"country,omitempty"`
	Locale             string                 `json:"locale,omitempty"`
	AppVersion         string                 `json:"appVersion,omitempty"`
	Custom             map[string]interface{} `json:"custom,omitempty"`
	PrivateAttributes  map[string]interface{} `json:"privateAttributes,omitempty"`
	StatsigEnvironment map[string]string      `json:"statsigEnvironment,omitempty"`
	CustomIDs          map[string]string      `json:"customIDs"`
}

func (user *User) getCopyForLogging() *User {
	copy := *user
	copy.PrivateAttributes = make(map[string]interface{})
	return &copy
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
	Name              string                 `json:"name"`
	Value             map[string]interface{} `json:"value"`
	RuleID            string                 `json:"rule_id"`
	IDType            string                 `json:"id_type"`
	GroupName         string                 `json:"group_name"`
	EvaluationDetails *EvaluationDetails     `json:"evaluation_details"`
}

type FeatureGate struct {
	Name              string             `json:"name"`
	Value             bool               `json:"value"`
	RuleID            string             `json:"rule_id"`
	IDType            string             `json:"id_type"`
	GroupName         string             `json:"group_name"`
	EvaluationDetails *EvaluationDetails `json:"evaluation_details"`
}

// A json blob configured in the Statsig Console
type DynamicConfig struct {
	configBase
}

type Layer struct {
	configBase
	LogExposure             *func(Layer, string) `json:"log_exposure"`
	AllocatedExperimentName string               `json:"allocated_experiment_name"`
}

func NewGate(name string, value bool, ruleID string, groupName string, idType string, evaluationDetails *EvaluationDetails) *FeatureGate {
	return &FeatureGate{
		Name:              name,
		Value:             value,
		RuleID:            ruleID,
		IDType:            idType,
		GroupName:         groupName,
		EvaluationDetails: evaluationDetails,
	}
}

func NewConfig(name string, value map[string]interface{}, ruleID string, idType string, groupName string, evaluationDetails *EvaluationDetails) *DynamicConfig {
	if value == nil {
		value = make(map[string]interface{})
	}
	return &DynamicConfig{
		configBase: configBase{
			Name:              name,
			Value:             value,
			RuleID:            ruleID,
			IDType:            idType,
			GroupName:         groupName,
			EvaluationDetails: evaluationDetails,
		},
	}
}

func NewLayer(name string, value map[string]interface{}, ruleID string, idType string, groupName string, logExposure *func(Layer, string), allocatedExperimentName string) *Layer {
	if value == nil {
		value = make(map[string]interface{})
	}
	return &Layer{
		configBase: configBase{
			Name:      name,
			Value:     value,
			RuleID:    ruleID,
			IDType:    idType,
			GroupName: groupName,
		},
		AllocatedExperimentName: allocatedExperimentName,
		LogExposure:             logExposure,
	}
}

// Gets the string value at the given key in the DynamicConfig
// Returns the fallback string if the item at the given key is not found or not of type string
func (d *configBase) GetString(key string, fallback string) string {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		}
	}

	return fallback
}

// Gets the string value at the given key in the DynamicConfig
// Returns the fallback string if the item at the given key is not found or not of type string
func (d *Layer) GetString(key string, fallback string) string {
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
			return val
		}
	}
	return fallback
}

// Gets the float64 value at the given key in the DynamicConfig
// Returns the fallback float64 if the item at the given key is not found or not of type float64
func (d *Layer) GetNumber(key string, fallback float64) float64 {
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
			return val
		}
	}
	return fallback
}

// Gets the boolean value at the given key in the DynamicConfig
// Returns the fallback boolean if the item at the given key is not found or not of type boolean
func (d *Layer) GetBool(key string, fallback bool) bool {
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
			return val
		}
	}
	return fallback
}

// Gets the slice value at the given key in the DynamicConfig
// Returns the fallback slice if the item at the given key is not found or not of type slice
func (d *Layer) GetSlice(key string, fallback []interface{}) []interface{} {
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
			return val
		}
	}
	return fallback
}

func (d *Layer) GetMap(key string, fallback map[string]interface{}) map[string]interface{} {
	if v, ok := d.Value[key]; ok {
		switch val := v.(type) {
		case map[string]interface{}:
			logExposure(d, key)
			return val
		}
	}
	return fallback
}

func logExposure(c *Layer, parameterName string) {
	if c == nil || c.LogExposure == nil {
		return
	}

	l := *c.LogExposure
	l(*c, parameterName)
}
