package types

type StatsigUser struct {
	UserID        string `json:"userID"`
	Email         string `json:"email"`
	IpAddress     string `json:"ip"`
	UserAgent     string `json:"userAgent"`
	Country       string `json:"country"`
	Locale        string `json:"locale"`
	ClientVersion string `json:"clientVersion"`
	Custom        string `json:"custom"`
}
