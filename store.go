package statsig

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

type configSpec struct {
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Salt              string          `json:"salt"`
	Enabled           bool            `json:"enabled"`
	Rules             []configRule    `json:"rules"`
	DefaultValue      json.RawMessage `json:"defaultValue"`
	IDType            string          `json:"idType"`
	ExplicitParamters []string        `json:"explicitParameters"`
}

type configRule struct {
	Name           string            `json:"name"`
	ID             string            `json:"id"`
	Salt           string            `json:"salt"`
	PassPercentage float64           `json:"passPercentage"`
	Conditions     []configCondition `json:"conditions"`
	ReturnValue    json.RawMessage   `json:"returnValue"`
	IDType         string            `json:"idType"`
	ConfigDelegate string            `json:"configDelegate"`
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
	LayerConfigs   []configSpec    `json:"layer_configs"`
	IDLists        map[string]bool `json:"id_lists"`
}

type downloadConfigsInput struct {
	SinceTime       int64           `json:"sinceTime"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type idList struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	CreationTime int64  `json:"creationTime"`
	URL          string `json:"url"`
	FileID       string `json:"fileID"`
	ids          sync.Map
}

type getIDListsInput struct {
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type store struct {
	featureGates       map[string]configSpec
	dynamicConfigs     map[string]configSpec
	layerConfigs       map[string]configSpec
	configsLock        sync.RWMutex
	idLists            map[string]*idList
	idListsLock        sync.RWMutex
	lastSyncTime       int64
	transport          *transport
	configSyncInterval time.Duration
	idListSyncInterval time.Duration
	shutdown           bool
}

func newStore(transport *transport) *store {
	return newStoreInternal(transport, 10*time.Second, time.Minute)
}

func newStoreInternal(transport *transport, configSyncInterval time.Duration, idListSyncInterval time.Duration) *store {
	store := &store{
		featureGates:       make(map[string]configSpec),
		dynamicConfigs:     make(map[string]configSpec),
		idLists:            make(map[string]*idList),
		transport:          transport,
		configSyncInterval: configSyncInterval,
		idListSyncInterval: idListSyncInterval,
	}
	store.fetchConfigSpecs()
	store.syncIDLists()
	go store.pollForRulesetChanges()
	go store.pollForIDListChanges()
	return store
}

func (s *store) getGate(name string) (configSpec, bool) {
	s.configsLock.RLock()
	gate, ok := s.featureGates[name]
	s.configsLock.RUnlock()
	return gate, ok
}

func (s *store) getDynamicConfig(name string) (configSpec, bool) {
	s.configsLock.RLock()
	config, ok := s.dynamicConfigs[name]
	s.configsLock.RUnlock()
	return config, ok
}

func (s *store) getLayerConfig(name string) (configSpec, bool) {
	s.configsLock.RLock()
	config, ok := s.layerConfigs[name]
	s.configsLock.RUnlock()
	return config, ok
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

		newLayers := make(map[string]configSpec)
		for _, layer := range specs.LayerConfigs {
			newLayers[layer.Name] = layer
		}

		s.configsLock.Lock()
		s.featureGates = newGates
		s.dynamicConfigs = newConfigs
		s.layerConfigs = newLayers
		s.configsLock.Unlock()
	}
}

func (s *store) getIDList(name string) *idList {
	s.idListsLock.RLock()
	list, ok := s.idLists[name]
	s.idListsLock.RUnlock()
	if ok {
		return list
	}
	return nil
}

func (s *store) deleteIDList(name string) {
	s.idListsLock.Lock()
	delete(s.idLists, name)
	s.idListsLock.Unlock()
}

func (s *store) syncIDLists() {
	var serverLists map[string]idList
	err := s.transport.postRequest("/get_id_lists", getIDListsInput{StatsigMetadata: s.transport.metadata}, &serverLists)
	if err != nil {
		return
	}

	wg := sync.WaitGroup{}
	for name, serverList := range serverLists {
		localList := s.getIDList(name)
		if localList == nil {
			localList = &idList{Name: name}
			s.idListsLock.Lock()
			s.idLists[name] = localList
			s.idListsLock.Unlock()
		}

		// skip if server list is invalid
		if serverList.URL == "" || serverList.CreationTime < localList.CreationTime || serverList.FileID == "" {
			continue
		}

		// reset the local list if returns server list has a newer file
		if serverList.FileID != localList.FileID && serverList.CreationTime >= localList.CreationTime {
			localList.URL = serverList.URL
			localList.FileID = serverList.FileID
			localList.CreationTime = serverList.CreationTime
			localList.Size = 0
			localList.ids = sync.Map{}
		}

		// skip if server list is not bigger
		if serverList.Size <= localList.Size {
			continue
		}

		wg.Add(1)
		go func(name string, l *idList) {
			defer wg.Done()
			res, err := s.transport.get(l.URL, map[string]string{"Range": fmt.Sprintf("bytes=%d-", l.Size)})
			if err != nil || res == nil {
				return
			}
			defer res.Body.Close()

			length, err := strconv.Atoi(res.Header.Get("content-length"))
			if err != nil || length <= 0 {
				return
			}

			bodyBytes, err := io.ReadAll(res.Body)
			if err != nil {
				return
			}
			content := string(bodyBytes)
			if len(content) <= 1 || (string(content[0]) != "-" && string(content[0]) != "+") {
				s.deleteIDList(name)
				return
			}

			lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if len(line) <= 1 {
					continue
				}
				id := line[1:]
				op := string(line[0])
				if op == "+" {
					l.ids.Store(id, true)
				} else if op == "-" {
					l.ids.Delete(id)
				}
			}
			l.Size = l.Size + int64(length)
		}(name, localList)
	}
	wg.Wait()
	for name := range s.idLists {
		if _, ok := serverLists[name]; !ok {
			s.deleteIDList(name)
		}
	}
}

func (s *store) pollForIDListChanges() {
	time.Sleep(s.idListSyncInterval)
	if s.shutdown {
		return
	}
	s.syncIDLists()
	s.pollForIDListChanges()
}

func (s *store) pollForRulesetChanges() {
	time.Sleep(s.configSyncInterval)
	if s.shutdown {
		return
	}
	s.fetchConfigSpecs()
	s.pollForRulesetChanges()
}

func (s *store) stopPolling() {
	s.shutdown = true
}
