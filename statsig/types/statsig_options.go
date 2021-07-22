package types

// Advanced options for configuring the Statsig SDK
type StatsigOptions struct {
	API         string
	Environment StatsigEnvironment
}

// See https://docs.statsig.com/guides/usingEnvironments
type StatsigEnvironment struct {
	Tier   string
	Params map[string]string
}
