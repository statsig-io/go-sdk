package types

type StatsigOptions struct {
	API         string
	Environment StatsigEnvironment
}

type StatsigEnvironment struct {
	Tier   string
	Params map[string]string
}
