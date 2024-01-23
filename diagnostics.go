package statsig

import (
	"math"
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
}

func newDiagnostics(options *Options) *diagnostics {
	return &diagnostics{
		initDiagnostics: &diagnosticsBase{
			context: InitializeContext,
			markers: make([]marker, 0),
			options: options,
		},
		syncDiagnostics: &diagnosticsBase{
			context: ConfigSyncContext,
			markers: make([]marker, 0),
			options: options,
		},
		apiDiagnostics: &diagnosticsBase{
			context: ApiCallContext,
			markers: make([]marker, 0),
			options: options,
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

func (d *diagnosticsBase) serializeWithSampling() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	markers := make([]marker, 0)
	sampledKeys := make(map[string]bool)
	for _, marker := range d.markers {
		markerKey := string(*marker.Key)
		if _, exists := sampledKeys[markerKey]; !exists {
			sampleRate, exists := d.samplingRates[markerKey]
			if !exists {
				sampledKeys[markerKey] = false
			} else {
				sampledKeys[markerKey] = sample(sampleRate)
			}
		}
		if !sampledKeys[markerKey] {
			markers = append(markers, marker)
		}
	}
	return map[string]interface{}{
		"context": d.context,
		"markers": markers,
	}
}

func (d *diagnosticsBase) updateSamplingRates(samplingRates map[string]int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.samplingRates = samplingRates
}

func sample(rate_over_ten_thousand int) bool {
	rand.Seed(time.Now().UnixNano())
	return int(math.Floor(rand.Float64()*10_000)) < rate_over_ten_thousand
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
	if *m.Key == OverallKey {
		if *m.Action == StartAction {
			msg = "Starting..."
		} else if *m.Action == EndAction {
			msg = "Done"
		}
	} else if *m.Key == DownloadConfigSpecsKey {
		if *m.Step == NetworkRequestStep {
			if *m.Action == StartAction {
				msg = "Loading specs from network..."
			} else if *m.Action == EndAction {
				if *m.Success {
					msg = "Done loading specs from network"
				} else {
					msg = "Failed to load specs from network"
				}
			}
		} else if *m.Step == ProcessStep {
			if *m.Action == StartAction {
				msg = "Processing specs from network..."
			} else if *m.Action == EndAction {
				if *m.Success {
					msg = "Done processing specs from network"
				} else {
					msg = "No updates to specs from network"
				}
			}
		}
	} else if *m.Key == DataStoreConfigSpecsKey {
		if *m.Step == FetchStep {
			if *m.Action == StartAction {
				msg = "Loading specs from adapter..."
			} else if *m.Action == EndAction {
				if *m.Success {
					msg = "Done loading specs from adapter"
				} else {
					msg = "Failed to load specs from adapter"
				}
			}
		} else if *m.Step == ProcessStep {
			if *m.Action == StartAction {
				msg = "Processing specs from adapter..."
			} else if *m.Action == EndAction {
				if *m.Success {
					msg = "Done processing specs from adapter"
				} else {
					msg = "No updates to specs from adapter"
				}
			}
		}
	}
	m.diagnostics.logProcess(msg)
}
