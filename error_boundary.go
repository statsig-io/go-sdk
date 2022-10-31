package statsig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

type errorBoundary struct {
	endpoint string `default:"https://statsigapi.net/v1/sdk_exception"`
	client   *http.Client
	seen     map[string]bool
}

type logExceptionRequestBody struct {
	Exception string `json:"exception"`
	Info      string `json:"info"`
}

type logExceptionResponse struct {
	Success bool
}

const (
	InvalidSDKKeyError  string = "Must provide a valid SDK key."
	EmptyUserError      string = "A non-empty StatsigUser.UserID or StatsigUser.CustomIDs is required. See https://docs.statsig.com/messages/serverRequiredUserID"
	EventBatchSizeError string = "The max number of events supported in one batch is 500. Please reduce the slice size and try again."
)

func newErrorBoundary(options *Options) *errorBoundary {
	errorBoundary := &errorBoundary{
		client: &http.Client{},
		seen:   make(map[string]bool),
	}
	if options.API != "" {
		errorBoundary.endpoint = options.API
	}
	return errorBoundary
}

func (e *errorBoundary) logException(exception error) {
	var exceptionString string
	if exception == nil {
		exceptionString = "Unknown"
	} else {
		exceptionString = exception.Error()
	}
	stack := make([]byte, 1024)
	runtime.Stack(stack, false)
	body := &logExceptionRequestBody{
		Exception: exceptionString,
		Info:      string(stack),
	}
	if e.seen[exceptionString] {
		return
	}
	e.seen[exceptionString] = true
	bodyString, err := json.Marshal(body)
	if err != nil {
		return
	}
	metadata := getStatsigMetadata()

	req, err := http.NewRequest("POST", e.endpoint, bytes.NewBuffer(bodyString))
	if err != nil {
		return
	}
	client := http.Client{}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(time.Now().Unix()*1000, 10))
	req.Header.Add("STATSIG-SDK-TYPE", metadata.SDKType)
	req.Header.Add("STATSIG-SDK-VERSION", metadata.SDKVersion)

	_, _ = client.Do(req)
}
