package types

// User specific attributes for evaluating Feature Gates, Experiments, and DyanmicConfigs
// NOTE: UserID is **required** - see https://docs.statsig.com/messages/serverRequiredUserID
type StatsigUser struct {
	UserID             string                 `json:"userID"`
	Email              string                 `json:"email"`
	IpAddress          string                 `json:"ip"`
	UserAgent          string                 `json:"userAgent"`
	Country            string                 `json:"country"`
	Locale             string                 `json:"locale"`
	AppVersion         string                 `json:"appVersion"`
	Custom             map[string]interface{} `json:"custom"`
	StatsigEnvironment map[string]string      `json:"statsigEnvironment"`
}
