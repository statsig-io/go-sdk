package evaluation

import (
	"encoding/json"
	"statsig/internal/net"
	"strconv"
)

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

type DownloadConfigsInput struct {
	SinceTime       string              `json:"sinceTime"`
	StatsigMetadata net.StatsigMetadata `json:"statsigMetadata"`
}

type Store struct {
	FeatureGates   map[string]ConfigSpec
	DynamicConfigs map[string]ConfigSpec
	lastSyncTime   int64
	network        *net.Net
}

func initStore(n *net.Net) *Store {
	store := &Store{
		FeatureGates:   make(map[string]ConfigSpec),
		DynamicConfigs: make(map[string]ConfigSpec),
		network:        n,
	}

	specs := store.fetchConfigSpecs()
	if specs.HasUpdates {
		newGates := make(map[string]ConfigSpec)
		for _, gate := range specs.FeatureGates {
			newGates[gate.Name] = gate
		}

		newConfigs := make(map[string]ConfigSpec)
		for _, config := range specs.FeatureGates {
			newConfigs[config.Name] = config
		}

		store.FeatureGates = newGates
		store.DynamicConfigs = newConfigs
	}

	return store
}

func (s *Store) fetchConfigSpecs() DownloadConfigSpecResponse {
	input := &DownloadConfigsInput{
		SinceTime:       strconv.FormatInt(s.lastSyncTime, 10),
		StatsigMetadata: s.network.GetStatsigMetadata(),
	}
	var specs DownloadConfigSpecResponse
	s.network.PostRequest("download_config_specs", input, &specs)
	s.lastSyncTime = specs.Time
	return specs
}
