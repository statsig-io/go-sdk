package statsig

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type DiagnosticsContext string

const (
	InitializeContext DiagnosticsContext = "initialize"
	ConfigSyncContext DiagnosticsContext = "config_sync"
	ApiCallContext    DiagnosticsContext = "api_call"
)

type DiagnosticsKey string

const (
	DownloadConfigSpecsKey  DiagnosticsKey = "download_config_specs"
	BootstrapKey            DiagnosticsKey = "bootstrap"
	GetIDListSourcesKey     DiagnosticsKey = "get_id_list_sources"
	GetIDListKey            DiagnosticsKey = "get_id_list"
	OverallKey              DiagnosticsKey = "overall"
	DataStoreConfigSpecsKey DiagnosticsKey = "data_store_config_specs"
	DataStoreIDLists        DiagnosticsKey = "data_store_id_lists"
	DataStoreIDList         DiagnosticsKey = "data_store_id_list"
	CheckGateApiKey         DiagnosticsKey = "check_gate"
	GetConfigApiKey         DiagnosticsKey = "get_config"
	GetLayerApiKey          DiagnosticsKey = "get_layer"
)

type DiagnosticsStep string

const (
	NetworkRequestStep DiagnosticsStep = "network_request"
	FetchStep          DiagnosticsStep = "fetch"
	ProcessStep        DiagnosticsStep = "process"
)

type DiagnosticsAction string

const (
	StartAction DiagnosticsAction = "start"
	EndAction   DiagnosticsAction = "end"
)

const MaxMarkerSize = 50

type diagnosticsBase struct {
	context       DiagnosticsContext
	markers       []marker
	mu            sync.RWMutex
	samplingRates map[string]int
	options       *Options
}

type diagnostics struct {
	initDiagnostics *diagnosticsBase
	syncDiagnostics *diagnosticsBase
	apiDiagnostics  *diagnosticsBase
}

type marker struct {
	Key       *DiagnosticsKey    `json:"key,omitempty"`
	Step      *DiagnosticsStep   `json:"step,omitempty"`
	Action    *DiagnosticsAction `json:"action,omitempty"`
	Timestamp int64              `json:"timestamp"`
	tags
	diagnostics *diagnosticsBase
}

type tags struct {
	Success     *bool   `json:"success,omitempty"`
	StatusCode  *int    `json:"statusCode,omitempty"`
	SDKRegion   *string `json:"sdkRegion,omitempty"`
	IDListCount *int    `json:"idListCount,omitempty"`
	URL         *string `json:"url,omitempty"`
	Name        *string `json:"name,omitempty"`
	Reason      *string `json:"reason,omitempty"`
}

var DEFAULT_SAMPLING_RATES = map[string]int{
	"initialize":  10000,
	"config_sync": 0,
	"api_call":    0,
}

func newDiagnostics(options *Options) *diagnostics {
	return &diagnostics{
		initDiagnostics: &diagnosticsBase{
			context:       InitializeContext,
			markers:       make([]marker, 0),
			options:       options,
			samplingRates: DEFAULT_SAMPLING_RATES,
		},
		syncDiagnostics: &diagnosticsBase{
			context:       ConfigSyncContext,
			markers:       make([]marker, 0),
			options:       options,
			samplingRates: DEFAULT_SAMPLING_RATES,
		},
		apiDiagnostics: &diagnosticsBase{
			context:       ApiCallContext,
			markers:       make([]marker, 0),
			options:       options,
			samplingRates: DEFAULT_SAMPLING_RATES,
		},
	}
}

func (d *diagnosticsBase) logProcess(msg string) {
	var process StatsigProcess
	switch d.context {
	case InitializeContext:
		process = StatsigProcessInitialize
	case ConfigSyncContext:
		process = StatsigProcessSync
	}
	Logger().LogStep(process, msg)
}

func (d *diagnosticsBase) serializeWithSampling() (map[string]interface{}, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	samplingRate, hasSamplingRate := d.samplingRates[string(d.context)]
	if !hasSamplingRate || len(d.markers) == 0 {
		return map[string]interface{}{}, false
	}
	shouldSample := sample(samplingRate)
	if !shouldSample {
		return map[string]interface{}{}, false
	}

	return map[string]interface{}{
		"context": d.context,
		"markers": d.markers,
	}, true
}

func (d *diagnosticsBase) updateSamplingRates(samplingRates map[string]int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.samplingRates = samplingRates
}

func sample(rate_over_ten_thousand int) bool {
	return int(rand.Float64()*10_000) < rate_over_ten_thousand
}

func (d *diagnosticsBase) clearMarkers() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.markers = nil
}

func (d *diagnosticsBase) isDisabled() bool {
	options := d.options.StatsigLoggerOptions
	return (options.DisableInitDiagnostics && d.context == InitializeContext) ||
		(options.DisableSyncDiagnostics && d.context == ConfigSyncContext) ||
		(options.DisableApiDiagnostics && d.context == ApiCallContext)
}

/* Context */
func (d *diagnostics) initialize() *marker {
	return &marker{diagnostics: d.initDiagnostics}
}

func (d *diagnostics) configSync() *marker {
	return &marker{diagnostics: d.syncDiagnostics}
}

func (d *diagnostics) api() *marker {
	return &marker{diagnostics: d.apiDiagnostics}
}

/* Keys */
func (m *marker) downloadConfigSpecs() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = DownloadConfigSpecsKey
	return m
}

func (m *marker) bootstrap() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = BootstrapKey
	return m
}

func (m *marker) getIdListSources() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = GetIDListSourcesKey
	return m
}

func (m *marker) getIdList() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = GetIDListKey
	return m
}

func (m *marker) overall() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = OverallKey
	return m
}

func (m *marker) dataStoreConfigSpecs() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = DataStoreConfigSpecsKey
	return m
}

func (m *marker) dataStoreIDLists() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = DataStoreIDLists
	return m
}

func (m *marker) dataStoreIDList() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = DataStoreIDList
	return m
}

func (m *marker) checkGate() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = CheckGateApiKey
	return m
}

func (m *marker) getConfig() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = GetConfigApiKey
	return m
}

func (m *marker) getLayer() *marker {
	m.Key = new(DiagnosticsKey)
	*m.Key = GetLayerApiKey
	return m
}

/* Steps */
func (m *marker) networkRequest() *marker {
	m.Step = new(DiagnosticsStep)
	*m.Step = NetworkRequestStep
	return m
}

func (m *marker) fetch() *marker {
	m.Step = new(DiagnosticsStep)
	*m.Step = FetchStep
	return m
}

func (m *marker) process() *marker {
	m.Step = new(DiagnosticsStep)
	*m.Step = ProcessStep
	return m
}

/* Actions */
func (m *marker) start() *marker {
	m.Action = new(DiagnosticsAction)
	*m.Action = StartAction
	return m
}

func (m *marker) end() *marker {
	m.Action = new(DiagnosticsAction)
	*m.Action = EndAction
	return m
}

/* Tags */
func (m *marker) success(val bool) *marker {
	m.Success = new(bool)
	*m.Success = val
	return m
}

func (m *marker) statusCode(val int) *marker {
	m.StatusCode = new(int)
	*m.StatusCode = val
	return m
}

func (m *marker) sdkRegion(val string) *marker {
	m.SDKRegion = new(string)
	*m.SDKRegion = val
	return m
}

func (m *marker) idListCount(val int) *marker {
	m.IDListCount = new(int)
	*m.IDListCount = val
	return m
}

func (m *marker) url(val string) *marker {
	m.URL = new(string)
	*m.URL = val
	return m
}

func (m *marker) name(val string) *marker {
	m.Name = new(string)
	*m.Name = val
	return m
}

func (m *marker) reason(reason string) *marker {
	m.Reason = new(string)
	*m.Reason = reason
	return m
}

/* End of chain */
func (m *marker) mark() {
	m.Timestamp = time.Now().UnixNano() / 1000000.0
	m.diagnostics.mu.Lock()
	defer m.diagnostics.mu.Unlock()
	if len(m.diagnostics.markers) >= MaxMarkerSize || m.diagnostics.isDisabled() {
		return
	}
	m.diagnostics.markers = append(m.diagnostics.markers, *m)
	m.logProcess()
}

func (m *marker) logProcess() {
	var msg string
	var dataSource string
	var dataType string
	switch key := *m.Key; key {
	case BootstrapKey:
		dataType = "specs"
		dataSource = "bootstrap"
	case DownloadConfigSpecsKey:
		dataType = "specs"
		dataSource = "network"
	case DataStoreConfigSpecsKey:
		dataType = "specs"
		dataSource = "adapter"
	case GetIDListSourcesKey:
		dataType = "list of id lists"
		dataSource = "network"
	case DataStoreIDLists:
		dataType = "list of id lists"
		dataSource = "adapter"
	case GetIDListKey:
		dataType = fmt.Sprintf("id list (%s)", *m.Name)
		dataSource = "network"
	case DataStoreIDList:
		dataType = fmt.Sprintf("id list (%s)", *m.Name)
		dataSource = "adapter"
	case OverallKey:
		dataType = ""
		dataSource = ""
	case CheckGateApiKey:
		fallthrough
	case GetConfigApiKey:
		fallthrough
	case GetLayerApiKey:
		fallthrough
	default:
		return
	}

	if *m.Key == OverallKey {
	switch *m.Action {
	case StartAction:
		msg = "Starting..."
	case EndAction:
		msg = "Done"
	}
	} else {
		switch *m.Step {
		case NetworkRequestStep, FetchStep:
			switch *m.Action {
			case StartAction:
				msg = fmt.Sprintf("Loading %s from %s...", dataType, dataSource)
			case EndAction:
				if *m.Success {
					msg = fmt.Sprintf("Done loading %s from %s", dataType, dataSource)
				} else {
					msg = fmt.Sprintf("Failed to load %s from %s", dataType, dataSource)
				}
			}
		case ProcessStep:
			switch *m.Action {
			case StartAction:
				msg = fmt.Sprintf("Processing %s from %s", dataType, dataSource)
			case EndAction:
				if *m.Success {
					msg = fmt.Sprintf("Done processing %s from %s", dataType, dataSource)
				} else {
					if *m.Key == DownloadConfigSpecsKey || *m.Key == DataStoreConfigSpecsKey {
						msg = fmt.Sprintf("No updates to %s from %s", dataType, dataSource)
					} else {
						msg = fmt.Sprintf("Failed to process %s from %s", dataType, dataSource)
					}
				}
			}
		}
	}
	m.diagnostics.logProcess(msg)
}
