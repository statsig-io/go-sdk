package statsig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type errorBoundary struct {
	api         string
	endpoint    string
	sdkKey      string
	client      *http.Client
	seen        map[string]bool
	seenLock    sync.RWMutex
	diagnostics *diagnostics
	options     *Options
}

type logExceptionRequestBody struct {
	Exception       string          `json:"exception"`
	Info            string          `json:"info"`
	StatsigMetadata statsigMetadata `json:"statsigMetadata"`
}

type logExceptionResponse struct {
	Success bool
}

var ErrorBoundaryAPI = "https://statsigapi.net/v1"
var ErrorBoundaryEndpoint = "/sdk_exception"

const (
	InvalidSDKKeyError  string = "Must provide a valid SDK key."
	EmptyUserError      string = "A non-empty StatsigUser.UserID or StatsigUser.CustomIDs is required. See https://docs.statsig.com/messages/serverRequiredUserID"
	EventBatchSizeError string = "The max number of events supported in one batch is 500. Please reduce the slice size and try again."
)

func newErrorBoundary(sdkKey string, options *Options, diagnostics *diagnostics) *errorBoundary {
	errorBoundary := &errorBoundary{
		api:         ErrorBoundaryAPI,
		endpoint:    ErrorBoundaryEndpoint,
		sdkKey:      sdkKey,
		client:      &http.Client{Timeout: time.Second * 3},
		seen:        make(map[string]bool),
		diagnostics: diagnostics,
		options:     options,
	}
	if options.API != "" {
		errorBoundary.api = options.API
	}
	return errorBoundary
}

func (e *errorBoundary) checkSeen(exceptionString string) bool {
	e.seenLock.Lock()
	defer e.seenLock.Unlock()
	if e.seen[exceptionString] {
		return true
	}
	e.seen[exceptionString] = true
	return false
}

func (e *errorBoundary) captureCheckGate(task func() FeatureGate) FeatureGate {
	defer e.ebRecover(func() {
		e.diagnostics.api().checkGate().end().success(false).mark()
	})
	e.diagnostics.api().checkGate().start().mark()
	res := task()
	e.diagnostics.api().checkGate().end().success(true).mark()
	return res
}

func (e *errorBoundary) captureGetConfig(task func() DynamicConfig) DynamicConfig {
	defer e.ebRecover(func() {
		e.diagnostics.api().getConfig().end().success(false).mark()
	})
	e.diagnostics.api().getConfig().start().mark()
	res := task()
	e.diagnostics.api().getConfig().end().success(true).mark()
	return res
}

func (e *errorBoundary) captureGetLayer(task func() Layer) Layer {
	defer e.ebRecover(func() {
		e.diagnostics.api().getLayer().end().success(false).mark()
	})
	e.diagnostics.api().getLayer().start().mark()
	res := task()
	e.diagnostics.api().getLayer().end().success(true).mark()
	return res
}

func (e *errorBoundary) captureGetClientInitializeResponse(task func() ClientInitializeResponse) ClientInitializeResponse {
	defer e.ebRecover(func() {})
	return task()
}

func (e *errorBoundary) captureGetUserPersistedValues(task func() UserPersistedValues) UserPersistedValues {
	defer e.ebRecover(func() {})
	return task()
}

func (e *errorBoundary) captureVoid(task func()) {
	defer e.ebRecover(func() {})
	task()
}

func (e *errorBoundary) captureGetExperimentLayer(task func() (string, bool)) (string, bool) {
	defer e.ebRecover(func() {})
	val, ok := task()
	return val, ok
}

func (e *errorBoundary) ebRecover(recoverCallback func()) {
	if err := recover(); err != nil {
		e.logException(toError(err))
		Logger().LogError(err)
		recoverCallback()
	}
}

func (e *errorBoundary) logException(exception error) {
	if e.options.StatsigLoggerOptions.DisableAllLogging || e.options.LocalMode {
		return
	}
	var exceptionString string
	if exception == nil {
		exceptionString = "Unknown"
	} else {
		exceptionString = exception.Error()
	}
	if e.checkSeen(exceptionString) {
		return
	}
	stack := make([]byte, 1024)
	runtime.Stack(stack, false)
	metadata := getStatsigMetadata()
	body := &logExceptionRequestBody{
		Exception:       exceptionString,
		Info:            string(stack),
		StatsigMetadata: metadata,
	}
	bodyString, err := json.Marshal(body)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", e.api+e.endpoint, bytes.NewBuffer(bodyString))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("STATSIG-API-KEY", e.sdkKey)
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(getUnixMilli(), 10))
	req.Header.Add("STATSIG-SDK-TYPE", metadata.SDKType)
	req.Header.Add("STATSIG-SDK-VERSION", metadata.SDKVersion)
	req.Header.Add("STATSIG-SERVER-SESSION-ID", metadata.SessionID)

	_, _ = e.client.Do(req)
}
