package statsig

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type configSpec struct {
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	Salt               string          `json:"salt"`
	Enabled            bool            `json:"enabled"`
	Rules              []configRule    `json:"rules"`
	DefaultValue       json.RawMessage `json:"defaultValue"`
	IDType             string          `json:"idType"`
	ExplicitParameters []string        `json:"explicitParameters"`
	Entity             string          `json:"entity"`
	IsActive           *bool           `json:"isActive,omitempty"`
	HasSharedParams    *bool           `json:"hasSharedParams,omitempty"`
}

type configRule struct {
	Name              string            `json:"name"`
	ID                string            `json:"id"`
	Salt              string            `json:"salt"`
	PassPercentage    float64           `json:"passPercentage"`
	Conditions        []configCondition `json:"conditions"`
	ReturnValue       json.RawMessage   `json:"returnValue"`
	IDType            string            `json:"idType"`
	ConfigDelegate    string            `json:"configDelegate"`
	IsExperimentGroup *bool             `json:"isExperimentGroup,omitempty"`
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
	HasUpdates     bool                `json:"has_updates"`
	Time           int64               `json:"time"`
	FeatureGates   []configSpec        `json:"feature_gates"`
	DynamicConfigs []configSpec        `json:"dynamic_configs"`
	LayerConfigs   []configSpec        `json:"layer_configs"`
	Layers         map[string][]string `json:"layers"`
	IDLists        map[string]bool     `json:"id_lists"`
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
	ids          *sync.Map
}

type getIDListsInput struct {
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type store struct {
	featureGates         map[string]configSpec
	dynamicConfigs       map[string]configSpec
	layerConfigs         map[string]configSpec
	experimentToLayer    map[string]string
	idLists              map[string]*idList
	lastSyncTime         int64
	initialSyncTime      int64
	initReason           evaluationReason
	transport            *transport
	configSyncInterval   time.Duration
	idListSyncInterval   time.Duration
	shutdown             bool
	rulesUpdatedCallback func(rules string, time int64)
	errorBoundary        *errorBoundary
	dataAdapter          IDataAdapter
	syncFailureCount     int
	mu                   sync.RWMutex
}

var syncOutdatedMax = 2 * time.Minute

func newStore(
	transport *transport,
	errorBoundary *errorBoundary,
	options *Options,
) *store {
	configSyncInterval := 10 * time.Second
	idListSyncInterval := time.Minute
	if options.ConfigSyncInterval > 0 {
		configSyncInterval = options.ConfigSyncInterval
	}
	if options.IDListSyncInterval > 0 {
		idListSyncInterval = options.IDListSyncInterval
	}
	return newStoreInternal(
		transport,
		configSyncInterval,
		idListSyncInterval,
		options.BootstrapValues,
		options.RulesUpdatedCallback,
		errorBoundary,
		options.DataAdapter,
	)
}

func newStoreInternal(
	transport *transport,
	configSyncInterval time.Duration,
	idListSyncInterval time.Duration,
	bootstrapValues string,
	rulesUpdatedCallback func(rules string, time int64),
	errorBoundary *errorBoundary,
	dataAdapter IDataAdapter,
) *store {
	store := &store{
		featureGates:         make(map[string]configSpec),
		dynamicConfigs:       make(map[string]configSpec),
		idLists:              make(map[string]*idList),
		transport:            transport,
		configSyncInterval:   configSyncInterval,
		idListSyncInterval:   idListSyncInterval,
		rulesUpdatedCallback: rulesUpdatedCallback,
		errorBoundary:        errorBoundary,
		initReason:           reasonUninitialized,
		dataAdapter:          dataAdapter,
		syncFailureCount:     0,
	}
	firstAttempt := true
	if dataAdapter != nil {
		firstAttempt = false
		dataAdapter.initialize()
		store.fetchConfigSpecsFromAdapter()
	} else if bootstrapValues != "" {
		firstAttempt = false
		specs := downloadConfigSpecResponse{}
		err := json.Unmarshal([]byte(bootstrapValues), &specs)
		if err == nil {
			store.setConfigSpecs(specs)
			store.mu.Lock()
			store.initReason = reasonBootstrap
			store.mu.Unlock()
		}
	}
	if store.lastSyncTime == 0 {
		if !firstAttempt {
			store.logProcess("Retrying with network...")
		}
		store.fetchConfigSpecsFromServer(true)
	}
	store.mu.Lock()
	store.initialSyncTime = store.lastSyncTime
	store.mu.Unlock()
	store.syncIDLists()
	go store.pollForRulesetChanges()
	go store.pollForIDListChanges()
	return store
}

func (s *store) getGate(name string) (configSpec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gate, ok := s.featureGates[name]
	return gate, ok
}

func (s *store) getDynamicConfig(name string) (configSpec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, ok := s.dynamicConfigs[name]
	return config, ok
}

func (s *store) getLayerConfig(name string) (configSpec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	layer, ok := s.layerConfigs[name]
	return layer, ok
}

func (s *store) getExperimentLayer(experimentName string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	layer, ok := s.experimentToLayer[experimentName]
	return layer, ok
}

func (s *store) fetchConfigSpecsFromAdapter() {
	s.logProcess("Loading specs from adapter...")
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "Error calling data adapter get: %s\n", err.(error).Error())
		}
	}()
	specString := s.dataAdapter.get(CONFIG_SPECS_KEY)
	specs := downloadConfigSpecResponse{}
	err := json.Unmarshal([]byte(specString), &specs)
	if err == nil {
		s.logProcess("Done loading specs")
		s.setConfigSpecs(specs)
		s.mu.Lock()
		s.initReason = reasonDataAdapter
		s.mu.Unlock()
	}
}

func (s *store) saveConfigSpecsToAdapter(specs downloadConfigSpecResponse) {
	specString, err := json.Marshal(specs)
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "Error calling data adapter set: %s\n", err.(error).Error())
		}
	}()
	if err == nil {
		s.dataAdapter.set(CONFIG_SPECS_KEY, string(specString))
	}
}

func (s *store) handleSyncError(err error, isColdStart bool) {
	s.syncFailureCount += 1
	failDuration := time.Duration(s.syncFailureCount) * s.configSyncInterval
	if isColdStart {
		fmt.Fprintf(os.Stderr, "Failed to initialize from the network. "+
			"See https://docs.statsig.com/messages/serverSDKConnection for more information\n")
		s.errorBoundary.logException(err)
	} else if failDuration > syncOutdatedMax {
		fmt.Fprintf(os.Stderr, "Syncing the server SDK with Statsig network has failed for %dms. "+
			"Your sdk will continue to serve gate/config/experiment definitions as of the last successful sync. "+
			"See https://docs.statsig.com/messages/serverSDKConnection for more information\n", int64(failDuration/time.Millisecond))
		s.errorBoundary.logException(err)
		s.syncFailureCount = 0
	}
}

func (s *store) fetchConfigSpecsFromServer(isColdStart bool) {
	s.logProcess("Loading specs from network...")
	s.mu.RLock()
	input := &downloadConfigsInput{
		SinceTime:       s.lastSyncTime,
		StatsigMetadata: s.transport.metadata,
	}
	s.mu.RUnlock()
	var specs downloadConfigSpecResponse
	err := s.transport.postRequest("/download_config_specs", input, &specs)
	if err != nil {
		s.handleSyncError(err, isColdStart)
		return
	}
	s.logProcess("Done loading specs")
	if s.setConfigSpecs(specs) {
		s.mu.Lock()
		s.initReason = reasonNetwork
		s.mu.Unlock()
		if s.rulesUpdatedCallback != nil {
			v, _ := json.Marshal(specs)
			s.rulesUpdatedCallback(string(v[:]), specs.Time)
		}
		if s.dataAdapter != nil {
			s.saveConfigSpecsToAdapter(specs)
		}
	}
}

func (s *store) setConfigSpecs(specs downloadConfigSpecResponse) bool {
	s.logProcess("Processing specs...")
	if specs.HasUpdates {
		// TODO: when adding eval details, differentiate REASON between bootstrap and network here
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

		newExperimentToLayer := make(map[string]string)
		for layerName, experiments := range specs.Layers {
			for _, experimentName := range experiments {
				newExperimentToLayer[experimentName] = layerName
			}
		}

		s.mu.Lock()
		s.featureGates = newGates
		s.dynamicConfigs = newConfigs
		s.layerConfigs = newLayers
		s.experimentToLayer = newExperimentToLayer
		s.lastSyncTime = specs.Time
		s.mu.Unlock()
		s.logProcess("Done processing specs")
		return true
	}
	s.logProcess("No updates to specs")
	return false
}

func (s *store) getIDList(name string) *idList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list, ok := s.idLists[name]
	if ok {
		return list
	}
	return nil
}

func (s *store) deleteIDList(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.idLists, name)
}

func (s *store) setIDList(name string, list *idList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idLists[name] = list
}

func (s *store) syncIDLists() {
	var serverLists map[string]idList
	err := s.transport.postRequest("/get_id_lists", getIDListsInput{StatsigMetadata: s.transport.metadata}, &serverLists)
	if err != nil {
		s.errorBoundary.logException(err)
		return
	}

	wg := sync.WaitGroup{}
	for name, serverList := range serverLists {
		localList := s.getIDList(name)
		if localList == nil {
			localList = &idList{Name: name}
			s.setIDList(name, localList)
		}

		// skip if server list is invalid
		if serverList.URL == "" || serverList.CreationTime < localList.CreationTime || serverList.FileID == "" {
			continue
		}

		// reset the local list if returns server list has a newer file
		if serverList.FileID != localList.FileID && serverList.CreationTime >= localList.CreationTime {
			localList = &idList{
				Name:         localList.Name,
				Size:         0,
				CreationTime: serverList.CreationTime,
				URL:          serverList.URL,
				FileID:       serverList.FileID,
				ids:          &sync.Map{},
			}
			s.setIDList(name, localList)
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
				s.errorBoundary.logException(err)
				return
			}
			defer res.Body.Close()

			length, err := strconv.Atoi(res.Header.Get("content-length"))
			if err != nil || length <= 0 {
				s.errorBoundary.logException(err)
				return
			}

			bodyBytes, err := io.ReadAll(res.Body)
			if err != nil {
				s.errorBoundary.logException(err)
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
			atomic.AddInt64((&l.Size), int64(length))
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
	for {
		time.Sleep(s.idListSyncInterval)
		stop := func() bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.shutdown
		}()
		if stop {
			break
		}
		s.syncIDLists()
	}
}

func (s *store) pollForRulesetChanges() {
	for {
		time.Sleep(s.configSyncInterval)
		stop := func() bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.shutdown
		}()
		if stop {
			break
		}
		if s.dataAdapter != nil && s.dataAdapter.shouldBeUsedForQueryingUpdates(CONFIG_SPECS_KEY) {
			s.fetchConfigSpecsFromAdapter()
		} else {
			s.fetchConfigSpecsFromServer(false)
		}
	}
}

func (s *store) stopPolling() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdown = true
}

func (s *store) logProcess(msg string) {
	var process StatsigProcess
	s.mu.RLock()
	if s.initReason == reasonUninitialized {
		process = StatsigProcessInitialize
	} else {
		process = StatsigProcessSync
	}
	s.mu.RUnlock()
	global.Logger().LogStep(process, msg)
}
