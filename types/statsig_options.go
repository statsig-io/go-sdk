package types

// Advanced options for configuring the Statsig SDK
type StatsigOptions struct {
	API         string             `json:"api"`
	Environment StatsigEnvironment `json:"environment"`
}

// See https://docs.statsig.com/guides/usingEnvironments
type StatsigEnvironment struct {
	Tier   string            `json:"tier"`
	Params map[string]string `json:"params"`
}
