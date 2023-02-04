package statsig

import (
	"runtime"
)

type statsigMetadata struct {
	SDKType         string `json:"sdkType"`
	SDKVersion      string `json:"sdkVersion"`
	LanguageVersion string `json:"languageVersion"`
}

func getStatsigMetadata() statsigMetadata {
	return statsigMetadata{
		SDKType:         "go-sdk",
		SDKVersion:      "1.8.1",
		LanguageVersion: runtime.Version()[2:],
	}
}
