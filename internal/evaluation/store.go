package evaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Store struct {
	FeatureGates   map[string]ConfigSpec
	DynamicConfigs map[string]ConfigSpec
	LastSyncTime   int64
}

type ConfigSpec struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Salt         string          `json:"salt"`
	Enabled      bool            `json:"enabled"`
	Rules        []ConfigRule    `json:"rules"`
	DefaultValue json.RawMessage `json:"defaultValue"`
}

type ConfigRule struct {
	Name           string            `json:"name"`
	ID             string            `json:"id"`
	PassPercentage float64           `json:"passPercentage"`
	Conditions     []ConfigCondition `json:"conditions"`
	ReturnValue    json.RawMessage   `json:"returnValue"`
}

type ConfigCondition struct {
	Type             string                 `json:"type"`
	Operator         string                 `json:"operator"`
	Field            string                 `json:"field"`
	TargetValue      interface{}            `json:"targetValue"`
	AdditionalValues map[string]interface{} `json:"additionalValues"`
}

type DownloadConfigSpecResponse struct {
	HasUpdates     bool         `json:"has_updates"`
	Time           int64        `json:"time"`
	FeatureGates   []ConfigSpec `json:"feature_gates"`
	DynamicConfigs []ConfigSpec `json:"dynamic_configs"`
}

type StatsigMetadata struct {
	SdkType    string `json:"sdkType"`
	SdkVersion string `json:"sdkVersion"`
}

type DownloadConfigsInput struct {
	SinceTime       string          `json:"sinceTime"`
	StatsigMetadata StatsigMetadata `json:"statsigMetadata"`
}

var sdkKey string
var lastSyncTime int64

func initStore(secret string) *Store {
	sdkKey = secret

	store := &Store{
		FeatureGates:   make(map[string]ConfigSpec),
		DynamicConfigs: make(map[string]ConfigSpec),
		LastSyncTime:   0,
	}

	go store.syncValues()

	return store
}

func (s *Store) syncValues() {
	// TODO: use prod instead of latest
	input := DownloadConfigsInput{SinceTime: strconv.FormatInt(lastSyncTime, 10), StatsigMetadata: StatsigMetadata{SdkType: "go-sdk", SdkVersion: "0.0.1"}}
	jsonStr, _ := json.Marshal(input)
	req, _ := http.NewRequest("POST", "https://latest.api.statsig.com/v1/download_config_specs", bytes.NewBuffer(jsonStr))
	req.Header.Add("STATSIG-API-KEY", sdkKey)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(time.Now().Unix(), 10))
	var client = &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		// TODO: retry and log
	}
	defer response.Body.Close()

	var specs DownloadConfigSpecResponse
	err = json.NewDecoder(response.Body).Decode(specs)

	if err != nil {
		// TODO: log
	} else {
		if specs.HasUpdates {
			newGates := make(map[string]ConfigSpec)
			for _, gate := range specs.FeatureGates {
				newGates[gate.Name] = gate
			}

			newConfigs := make(map[string]ConfigSpec)
			for _, config := range specs.FeatureGates {
				newConfigs[config.Name] = config
			}

			s.FeatureGates = newGates
			s.DynamicConfigs = newConfigs
			s.LastSyncTime = specs.Time
		}
	}
	fmt.Println("sync'ing")
	time.Sleep(1 * time.Second)
	s.syncValues()
}
