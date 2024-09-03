package statsig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type configSpec struct {
	Name               string                 `json:"name"`
	Type               string                 `json:"type"`
	Salt               string                 `json:"salt"`
	Enabled            bool                   `json:"enabled"`
	Rules              []configRule           `json:"rules"`
	DefaultValue       json.RawMessage        `json:"defaultValue"`
	DefaultValueJSON   map[string]interface{} `json:"-"`
	DefaultValueBool   *bool                  `json:"-"`
	IDType             string                 `json:"idType"`
	ExplicitParameters []string               `json:"explicitParameters"`
	Entity             string                 `json:"entity"`
	IsActive           *bool                  `json:"isActive,omitempty"`
	HasSharedParams    *bool                  `json:"hasSharedParams,omitempty"`
	TargetAppIDs       []string               `json:"targetAppIDs,omitempty"`
}

func (c configSpec) hasTargetAppID(appId string) bool {
	if appId == "" {
		return true
	}
	for _, e := range c.TargetAppIDs {
		if e == appId {
			return true
		}
	}
	return false
}

type configRule struct {
	Name              string                 `json:"name"`
	ID                string                 `json:"id"`
	GroupName         string                 `json:"groupName,omitempty"`
	Salt              string                 `json:"salt"`
	PassPercentage    float64                `json:"passPercentage"`
	Conditions        []configCondition      `json:"conditions"`
	ReturnValue       json.RawMessage        `json:"returnValue"`
	ReturnValueJSON   map[string]interface{} `json:"-"`
	ReturnValueBool   *bool                  `json:"-"`
	IDType            string                 `json:"idType"`
	ConfigDelegate    string                 `json:"configDelegate"`
	IsExperimentGroup *bool                  `json:"isExperimentGroup,omitempty"`
}

type configCondition struct {
	Type             string                 `json:"type"`
	Operator         string                 `json:"operator"`
	Field            string                 `json:"field"`
	TargetValue      interface{}            `json:"targetValue"`
	UserBucket       map[int64]bool         `json:"-"`
	AdditionalValues map[string]interface{} `json:"additionalValues"`
	IDType           string                 `json:"idType"`
}

type downloadConfigSpecResponse struct {
	HasUpdates             bool                `json:"has_updates"`
	Time                   int64               `json:"time"`
	FeatureGates           []configSpec        `json:"feature_gates"`
	DynamicConfigs         []configSpec        `json:"dynamic_configs"`
	LayerConfigs           []configSpec        `json:"layer_configs"`
	Layers                 map[string][]string `json:"layers"`
	IDLists                map[string]bool     `json:"id_lists"`
	DiagnosticsSampleRates map[string]int      `json:"diagnostics"`
	SDKKeysToAppID         map[string]string   `json:"sdk_keys_to_app_ids,omitempty"`
	HashedSDKKeysToAppID   map[string]string   `json:"hashed_sdk_keys_to_app_ids,omitempty"`
	HashedSDKKeyUsed       string              `json:"hashed_sdk_key_used,omitempty"`
	SDKFlags               map[string]bool     `json:"sdk_flags,omitempty"`
}

type idList struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	CreationTime int64  `json:"creationTime"`
	URL          string `json:"url"`
	FileID       string `json:"fileID"`
	ids          *sync.Map
	mu           *sync.RWMutex
}

type DataSource string

const (
	AdapterDataSource DataSource = "adapter"
	NetworkDataSource DataSource = "network"
)

type store struct {
	featureGates         map[string]configSpec
	dynamicConfigs       map[string]configSpec
	layerConfigs         map[string]configSpec
	experimentToLayer    map[string]string
	sdkKeysToAppID       map[string]string
	hashedSDKKeysToAppID map[string]string
	idLists              map[string]*idList
	lastSyncTime         int64
	initialSyncTime      int64
	initReason           evaluationReason
	initializedIDLists   bool
	transport            *transport
	configSyncInterval   time.Duration
	idListSyncInterval   time.Duration
	shutdown             bool
	rulesUpdatedCallback func(rules string, time int64)
	errorBoundary        *errorBoundary
	dataAdapter          IDataAdapter
	syncFailureCount     int
	diagnostics          *diagnostics
	mu                   sync.RWMutex
	sdkKey               string
	isPolling            bool
	bootstrapValues      string
}

var syncOutdatedMax = 2 * time.Minute

func newStore(
	transport *transport,
	errorBoundary *errorBoundary,
	options *Options,
	diagnostics *diagnostics,
	sdkKey string,
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
		options.RulesUpdatedCallback,
		errorBoundary,
		options.DataAdapter,
		diagnostics,
		sdkKey,
		options.BootstrapValues,
	)
}

func newStoreInternal(
	transport *transport,
	configSyncInterval time.Duration,
	idListSyncInterval time.Duration,
	rulesUpdatedCallback func(rules string, time int64),
	errorBoundary *errorBoundary,
	dataAdapter IDataAdapter,
	diagnostics *diagnostics,
	sdkKey string,
	bootstrapValues string,
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
		initializedIDLists:   false,
		dataAdapter:          dataAdapter,
		syncFailureCount:     0,
		diagnostics:          diagnostics,
		sdkKey:               sdkKey,
		isPolling:            false,
		bootstrapValues:      bootstrapValues,
	}
	return store
}

func (s *store) startPolling() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isPolling {
		go s.pollForRulesetChanges()
		go s.pollForIDListChanges()
		s.isPolling = true
	}
}

func (s *store) initialize() {
	firstAttempt := true
	if s.dataAdapter != nil {
		firstAttempt = false
		s.dataAdapter.Initialize()
		s.fetchConfigSpecsFromAdapter()
	} else if s.bootstrapValues != "" {
		firstAttempt = false
		if _, updated := s.processConfigSpecs(s.bootstrapValues, s.addDiagnostics().bootstrap()); updated {
			s.mu.Lock()
			s.initReason = reasonBootstrap
			s.mu.Unlock()
		}
	}
	if s.lastSyncTime == 0 {
		if !firstAttempt {
			s.diagnostics.initDiagnostics.logProcess("Retrying with network...")
		}
		s.fetchConfigSpecsFromServer(true)
	}
	s.mu.Lock()
	s.initialSyncTime = s.lastSyncTime
	s.mu.Unlock()
	if s.dataAdapter != nil {
		s.fetchIDListsFromAdapter()
	} else {
		s.fetchIDListsFromServer()
	}
	s.mu.Lock()
	s.initializedIDLists = true
	s.mu.Unlock()
	s.startPolling()
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

func (s *store) getAppIDForSDKKey(clientKey string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if appId, ok := s.hashedSDKKeysToAppID[getDJB2Hash(clientKey)]; ok {
		return appId, ok
	}
	appId, ok := s.sdkKeysToAppID[clientKey]
	return appId, ok
}

func (s *store) fetchConfigSpecsFromAdapter() {
	s.addDiagnostics().dataStoreConfigSpecs().fetch().start().mark()
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(fmt.Sprintf("Error calling data adapter get: %s\n", toError(err).Error()))
		}
	}()
	specString := s.dataAdapter.Get(CONFIG_SPECS_KEY)
	s.addDiagnostics().dataStoreConfigSpecs().fetch().end().success(true).mark()
	if _, updated := s.processConfigSpecs(specString, s.addDiagnostics().dataStoreConfigSpecs()); updated {
		s.mu.Lock()
		s.initReason = reasonDataAdapter
		s.mu.Unlock()
	}
}

func (s *store) saveConfigSpecsToAdapter(specs downloadConfigSpecResponse) {
	if s.dataAdapter == nil {
		return
	}
	specString, err := json.Marshal(specs)
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(fmt.Sprintf("Error calling data adapter set: %s\n", toError(err).Error()))
		}
	}()
	if err == nil {
		s.dataAdapter.Set(CONFIG_SPECS_KEY, string(specString))
	}
}

func (s *store) handleSyncError(err error, isColdStart bool) {
	s.syncFailureCount += 1
	failDuration := time.Duration(s.syncFailureCount) * s.configSyncInterval
	if isColdStart {
		Logger().LogError(fmt.Sprintf("Failed to initialize from the network. " +
			"See https://docs.statsig.com/messages/serverSDKConnection for more information\n"))
		s.errorBoundary.logException(err)
	} else if failDuration > syncOutdatedMax {
		Logger().LogError(fmt.Sprintf("Syncing the server SDK with Statsig network has failed for %dms. "+
			"Your sdk will continue to serve gate/config/experiment definitions as of the last successful sync. "+
			"See https://docs.statsig.com/messages/serverSDKConnection for more information\n", int64(failDuration/time.Millisecond)))
		s.errorBoundary.logException(err)
		s.syncFailureCount = 0
	}
}

func (s *store) fetchConfigSpecsFromServer(isColdStart bool) {
	if s.transport.options.LocalMode {
		return
	}
	var specs downloadConfigSpecResponse
	res, err := s.transport.download_config_specs(s.lastSyncTime, &specs, s.addDiagnostics())
	if res == nil || err != nil {
		s.handleSyncError(err, isColdStart)
		return
	}
	parsed, updated := s.processConfigSpecs(specs, s.addDiagnostics().downloadConfigSpecs())
	if parsed {
		s.mu.Lock()
		defer s.mu.Unlock()
		if updated {
			s.initReason = reasonNetwork
			if s.rulesUpdatedCallback != nil {
				v, _ := json.Marshal(specs)
				s.rulesUpdatedCallback(string(v[:]), specs.Time)
			}
			s.saveConfigSpecsToAdapter(specs)
		} else {
			s.initReason = reasonNetworkNotModified
		}
	}
}

func (s *store) processConfigSpecs(configSpecs interface{}, diagnosticsMarker *marker) (bool, bool) {
	diagnosticsMarker.process().start().mark()
	specs := downloadConfigSpecResponse{}
	parsed, updated := false, false
	switch specsTyped := configSpecs.(type) {
	case string:
		err := json.Unmarshal([]byte(specsTyped), &specs)
		if err == nil {
			parsed, updated = s.setConfigSpecs(specs)
		}
	case downloadConfigSpecResponse:
		parsed, updated = s.setConfigSpecs(specsTyped)
	default:
		parsed, updated = false, false
	}
	diagnosticsMarker.process().end().success(updated).mark()
	return parsed, updated
}

func (s *store) parseJSONValuesFromSpec(spec *configSpec) {
	var defaultValue map[string]interface{}
	err := json.Unmarshal(spec.DefaultValue, &defaultValue)
	if err != nil {
		defaultValue = make(map[string]interface{})
	}
	spec.DefaultValueJSON = defaultValue
	for i, rule := range spec.Rules {
		var ruleValue map[string]interface{}
		err := json.Unmarshal(rule.ReturnValue, &ruleValue)
		if err != nil {
			ruleValue = make(map[string]interface{})
		}
		spec.Rules[i].ReturnValueJSON = ruleValue
	}
}

func (s *store) parseTargetValueMapFromSpec(spec *configSpec) {
	for _, rule := range spec.Rules {
		for i, cond := range rule.Conditions {
			if (cond.Operator == "any" || cond.Operator == "none") && cond.Type == "user_bucket" {
				userBucketArray, ok := cond.TargetValue.([]interface{})
				if len(userBucketArray) == 0 || !ok {
					return
				}
				rule.Conditions[i].UserBucket = make(map[int64]bool)
				for _, val := range userBucketArray {
					rule.Conditions[i].UserBucket[int64(val.(float64))] = true
				}
			}
		}
	}
}

// Returns a tuple of booleans indicating 1. parsed, 2. updated
func (s *store) setConfigSpecs(specs downloadConfigSpecResponse) (bool, bool) {
	if specs.Time < s.lastSyncTime {
		return false, false
	}
	s.diagnostics.initDiagnostics.updateSamplingRates(specs.DiagnosticsSampleRates)
	s.diagnostics.syncDiagnostics.updateSamplingRates(specs.DiagnosticsSampleRates)
	s.diagnostics.apiDiagnostics.updateSamplingRates(specs.DiagnosticsSampleRates)

	if specs.HashedSDKKeyUsed != "" && specs.HashedSDKKeyUsed != getDJB2Hash(s.sdkKey) {
		s.errorBoundary.logException(fmt.Errorf("SDK key mismatch. Key used to generate response does not match key provided. Expected %s, got %s", getDJB2Hash(s.sdkKey), specs.HashedSDKKeyUsed))
		return false, false
	}

	if specs.HasUpdates {
		newGates := make(map[string]configSpec)
		for _, gate := range specs.FeatureGates {
			s.parseTargetValueMapFromSpec(&gate)
			newGates[gate.Name] = gate
		}

		newConfigs := make(map[string]configSpec)
		for _, config := range specs.DynamicConfigs {
			s.parseTargetValueMapFromSpec(&config)
			s.parseJSONValuesFromSpec(&config)
			newConfigs[config.Name] = config
		}

		newLayers := make(map[string]configSpec)
		for _, layer := range specs.LayerConfigs {
			s.parseTargetValueMapFromSpec(&layer)
			s.parseJSONValuesFromSpec(&layer)
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
		s.sdkKeysToAppID = specs.SDKKeysToAppID
		s.hashedSDKKeysToAppID = specs.HashedSDKKeysToAppID
		s.lastSyncTime = specs.Time
		s.mu.Unlock()
		return true, true
	}
	return true, false
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

func (s *store) fetchIDListsFromServer() {
	if s.transport.options.LocalMode {
		return
	}
	var serverLists map[string]idList
	res, err := s.transport.get_id_lists(&serverLists, s.addDiagnostics())
	if res == nil || err != nil {
		s.errorBoundary.logException(err)
		return
	}
	s.processIDListsFromNetwork(serverLists)
	s.saveIDListsToAdapter(s.idLists)
}

func (s *store) fetchIDListsFromAdapter() {
	s.addDiagnostics().dataStoreIDLists().fetch().start().mark()
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(fmt.Sprintf("Error calling data adapter get: %s\n", toError(err).Error()))
		}
	}()
	idListsString := s.dataAdapter.Get(ID_LISTS_KEY)
	var idLists map[string]idList
	err := json.Unmarshal([]byte(idListsString), &idLists)
	if err != nil {
		s.addDiagnostics().dataStoreIDLists().fetch().end().success(false).mark()
		return
	}
	s.addDiagnostics().dataStoreIDLists().fetch().end().success(true).mark()
	s.processIDListsFromAdapter(idLists)
}

func (s *store) saveIDListsToAdapter(idLists map[string]*idList) {
	if s.dataAdapter == nil {
		return
	}
	idListsJSON, err := json.Marshal(idLists)
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(fmt.Sprintf("Error calling data adapter set: %s\n", toError(err).Error()))
		}
	}()
	if err == nil {
		for name := range idLists {
			buf := new(bytes.Buffer)
			list := s.getIDList(name)
			list.mu.Lock()
			list.ids.Range(func(key, value interface{}) bool {
				buf.WriteString(fmt.Sprintf("+%s\n", key))
				return true
			})
			list.mu.Unlock()
			s.dataAdapter.Set(fmt.Sprintf("%s::%s", ID_LISTS_KEY, list.Name), buf.String())
		}
		s.dataAdapter.Set(ID_LISTS_KEY, string(idListsJSON))
	}
}

func (s *store) processIDListsFromNetwork(idLists map[string]idList) {
	s.addDiagnostics().getIdListSources().process().start().idListCount(len(idLists)).mark()
	s.processIDLists(idLists, NetworkDataSource)
	s.addDiagnostics().getIdListSources().process().end().success(true).idListCount(len(idLists)).mark()
}

func (s *store) processIDListsFromAdapter(idLists map[string]idList) {
	s.addDiagnostics().dataStoreIDLists().process().start().idListCount(len(idLists)).mark()
	s.processIDLists(idLists, AdapterDataSource)
	s.addDiagnostics().dataStoreIDLists().process().end().success(true).idListCount(len(idLists)).mark()
}

func (s *store) processIDLists(idLists map[string]idList, source DataSource) {
	wg := sync.WaitGroup{}
	for name, serverList := range idLists {
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
				mu:           &sync.RWMutex{},
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
			if source == NetworkDataSource {
				s.downloadSingleIDListFromServer(l)
			} else if source == AdapterDataSource {
				s.getSingleIDListFromAdapter(l)
			} else {
				s.errorBoundary.logException(errors.New("Invalid ID list data source"))
			}
		}(name, localList)
	}
	wg.Wait()
	for name := range s.idLists {
		if _, ok := idLists[name]; !ok {
			s.deleteIDList(name)
		}
	}
}

func (s *store) downloadSingleIDListFromServer(list *idList) {
	s.addDiagnostics().getIdList().networkRequest().start().name(list.Name).url(list.URL).mark()
	res, err := s.transport.get_id_list(list.URL, map[string]string{"Range": fmt.Sprintf("bytes=%d-", list.Size)})
	if err != nil || res == nil {
		marker := s.addDiagnostics().getIdList().networkRequest().end().name(list.Name).url(list.URL).success(false)
		if res != nil {
			marker.statusCode(res.StatusCode).sdkRegion(safeGetFirst(res.Header["X-Statsig-Region"]))
		}
		marker.mark()
		s.errorBoundary.logException(err)
		return
	}
	defer res.Body.Close()
	s.addDiagnostics().getIdList().networkRequest().end().name(list.Name).url(list.URL).
		success(true).statusCode(res.StatusCode).sdkRegion(safeGetFirst(res.Header["X-Statsig-Region"])).mark()
	s.processSingleIDListFromNetwork(list, res)
}

func (s *store) getSingleIDListFromAdapter(list *idList) {
	s.addDiagnostics().dataStoreIDList().fetch().start().name(list.Name).mark()
	defer func() {
		if err := recover(); err != nil {
			Logger().LogError(fmt.Sprintf("Error calling data adapter get: %s\n", toError(err).Error()))
		}
	}()
	content := s.dataAdapter.Get(fmt.Sprintf("%s::%s", ID_LISTS_KEY, list.Name))
	contentBytes := []byte(content)
	content = string(contentBytes[list.Size:])
	s.addDiagnostics().dataStoreIDList().fetch().end().name(list.Name).success(true).mark()
	s.processSingleIDListFromAdapter(list, content)
}

func (s *store) processSingleIDListFromNetwork(list *idList, res *http.Response) {
	s.addDiagnostics().getIdList().process().start().name(list.Name).url(list.URL).mark()
	length, err := strconv.Atoi(res.Header.Get("content-length"))
	if err != nil || length <= 0 {
		s.addDiagnostics().getIdList().process().end().name(list.Name).url(list.URL).success(false).mark()
		s.errorBoundary.logException(err)
		return
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		s.addDiagnostics().getIdList().process().end().name(list.Name).url(list.URL).success(false).mark()
		s.errorBoundary.logException(err)
		return
	}

	content := string(bodyBytes)
	if len(content) <= 1 || (string(content[0]) != "-" && string(content[0]) != "+") {
		s.addDiagnostics().getIdList().process().end().name(list.Name).url(list.URL).success(false).mark()
		s.deleteIDList(list.Name)
		return
	}
	s.processSingleIDList(list, content, length)
	s.addDiagnostics().getIdList().process().end().name(list.Name).url(list.URL).success(true).mark()
}

func (s *store) processSingleIDListFromAdapter(list *idList, content string) {
	s.addDiagnostics().dataStoreIDList().process().start().name(list.Name).url(list.URL).mark()
	s.processSingleIDList(list, content, len(content))
	s.addDiagnostics().dataStoreIDList().process().end().name(list.Name).url(list.URL).success(true).mark()
}

func (s *store) processSingleIDList(list *idList, content string, length int) {
	list.mu.Lock()
	defer list.mu.Unlock()
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) <= 1 {
			continue
		}
		id := line[1:]
		op := string(line[0])
		if op == "+" {
			list.ids.Store(id, true)
		} else if op == "-" {
			list.ids.Delete(id)
		}
	}
	atomic.AddInt64((&list.Size), int64(length))
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
		if s.dataAdapter != nil && s.dataAdapter.ShouldBeUsedForQueryingUpdates(ID_LISTS_KEY) {
			s.fetchIDListsFromAdapter()
		} else {
			s.fetchIDListsFromServer()
		}
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
		if s.dataAdapter != nil && s.dataAdapter.ShouldBeUsedForQueryingUpdates(CONFIG_SPECS_KEY) {
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
	s.isPolling = false
}

func (s *store) addDiagnostics() *marker {
	var marker *marker
	s.mu.RLock()
	if !s.initializedIDLists {
		marker = s.diagnostics.initialize()
	} else {
		marker = s.diagnostics.configSync()
	}
	s.mu.RUnlock()
	return marker
}
