package statsig

type StatsigContext struct {
	Caller       string
	EventCount   int
	ConfigName   string
	ClientKey    string
	Hash         string
	BypassDedupe bool
	TargetAppID  string
	LogToOutput  bool
}

func (sc *StatsigContext) getContextForLogging() map[string]interface{} {
	return map[string]interface{}{
		"tag":        sc.Caller,
		"eventCount": sc.EventCount,
		"configName": sc.ConfigName,
		"clientKey":  sc.ClientKey,
		"hash":       sc.ClientKey,
	}
}
