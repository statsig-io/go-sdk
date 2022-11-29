package statsig

type statsigMetadata struct {
	SDKType    string `json:"sdkType"`
	SDKVersion string `json:"sdkVersion"`
}

func getStatsigMetadata() statsigMetadata {
	return statsigMetadata{
		SDKType:    "go-sdk",
		SDKVersion: "1.6.2",
	}
}
