package statsig

type errorContext struct {
	evalContext  *evalContext
	Caller       string `json:"tag,omitempty"`
	BypassDedupe bool
	LogToOutput  bool
	EventCount   int
}

type evalContext struct {
	Caller                string `json:"tag,omitempty"`
	ConfigName            string `json:"configName,omitempty"`
	ClientKey             string `json:"clientKey,omitempty"`
	Hash                  string `json:"hash,omitempty"`
	TargetAppID           string
	IncludeLocalOverrides bool
	IsManualExposure      bool
	IsExperiment          bool
	DisableLogExposures   bool
	PersistedValues       UserPersistedValues
}
