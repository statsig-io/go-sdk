package statsig

import (
	"encoding/json"
	"sync"
	"time"
)

type configSpec struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Salt         string          `json:"salt"`
	Enabled      bool            `json:"enabled"`
	Rules        []configRule    `json:"rules"`
	DefaultValue json.RawMessage `json:"defaultValue"`
	IDType       string          `json:"idType"`
}

type configRule struct {
	Name           string            `json:"name"`
	ID             string            `json:"id"`
	Salt           string            `json:"salt"`
	PassPercentage float64           `json:"passPercentage"`
	Conditions     []configCondition `json:"conditions"`
	ReturnValue    json.RawMessage   `json:"returnValue"`
	IDType         string            `json:"idType"`
}

type configCondition struct {
	Type             string                 `json:"type"`
	Operator         string                 `json:"operator"`
	Field            string                 `json:"field"`
	TargetValue      interface{}            `json:"targetValue"`
	AdditionalValues map[string]interface{} `json:"additionalValues"`
	IDType           string                 `json:"idType"`
}

type downloadConfigSpecResponse struct {
	HasUpdates     bool            `json:"has_updates"`
	Time           int64           `json:"time"`
	FeatureGates   []configSpec    `json:"feature_gates"`
	DynamicConfigs []configSpec    `json:"dynamic_configs"`
	IDLists        map[string]bool `json:"id_lists"`
}

type downloadConfigsInput struct {
	SinceTime       int64           `json:"sinceTime"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type idList struct {
	ids  map[string]bool
	time int64
}

type downloadIDListInput struct {
	ListName        string          `json:"listName"`
	SinceTime       int64           `json:"sinceTime"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type downloadIDListResponse struct {
	AddIDs    []string `json:"add_ids"`
	RemoveIDs []string `json:"remove_ids"`
	Time      int64    `json:"time"`
}

type store struct {
	featureGates   map[string]configSpec
	dynamicConfigs map[string]configSpec
	idLists        map[string]idList
	lastSyncTime   int64
	transport      *transport
}

func newStore(transport *transport) *store {
	store := &store{
		featureGates:   make(map[string]configSpec),
		dynamicConfigs: make(map[string]configSpec),
		idLists:        make(map[string]idList),
		transport:      transport,
	}

	store.fetchConfigSpecs()
	store.syncIDLists()
	go store.pollForRulesetChanges()
	go store.pollForIDListChanges()
	return store
}

func (s *store) fetchConfigSpecs() {
	input := &downloadConfigsInput{
		SinceTime:       s.lastSyncTime,
		StatsigMetadata: s.transport.metadata,
	}
	var specs downloadConfigSpecResponse
	s.transport.postRequest("/download_config_specs", input, &specs)
	s.lastSyncTime = specs.Time
	if specs.HasUpdates {
		newGates := make(map[string]configSpec)
		for _, gate := range specs.FeatureGates {
			newGates[gate.Name] = gate
		}

		newConfigs := make(map[string]configSpec)
		for _, config := range specs.DynamicConfigs {
			newConfigs[config.Name] = config
		}

		s.featureGates = newGates
		s.dynamicConfigs = newConfigs

		for list := range specs.IDLists {
			if _, ok := s.idLists[list]; !ok {
				s.idLists[list] = idList{ids: make(map[string]bool), time: 0}
			}
		}
		for list := range s.idLists {
			if _, ok := specs.IDLists[list]; !ok {
				delete(s.idLists, list)
			}
		}
	}
}

func (s *store) syncIDLists() {
	wg := sync.WaitGroup{}
	for name, list := range s.idLists {
		wg.Add(1)
		go func(name string, list *idList) {
			var res downloadIDListResponse
			err := s.transport.postRequest(
				"/download_id_list",
				downloadIDListInput{ListName: name, SinceTime: list.time, StatsigMetadata: s.transport.metadata},
				&res)
			if err == nil {
				for _, id := range res.AddIDs {
					list.ids[id] = true
				}
				for _, id := range res.RemoveIDs {
					delete(list.ids, id)
				}
				list.time = res.Time
			}
		}(name, &list)
	}
	wg.Wait()
}

func (s *store) pollForIDListChanges() {
	time.Sleep(time.Minute)
	s.syncIDLists()
	s.pollForIDListChanges()
}

func (s *store) pollForRulesetChanges() {
	time.Sleep(10 * time.Second)
	s.fetchConfigSpecs()
	s.pollForRulesetChanges()
}
