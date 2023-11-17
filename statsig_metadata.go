package statsig

import (
	"runtime"
)

type statsigMetadata struct {
	SDKType         string `json:"sdkType"`
	SDKVersion      string `json:"sdkVersion"`
	LanguageVersion string `json:"languageVersion"`
	SessionID       string `json:"sessionID"`
}

func getStatsigMetadata() statsigMetadata {
	return statsigMetadata{
		SDKType:         "go-sdk",
		SDKVersion:      "1.14.0",
		LanguageVersion: runtime.Version()[2:],
		SessionID:       SessionID(),
	}
}
