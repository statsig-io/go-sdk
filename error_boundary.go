package statsig

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

type errorBoundary struct {
	endpoint string `default:"https://statsigapi.net/v1/sdk_exception"`
	client   *http.Client
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
	EmptyUserError      string = "A non-empty StatsigUser.UserID is required. See https://docs.statsig.com/messages/serverRequiredUserID"
	EventBatchSizeError string = "The max number of events supported in one batch is 500. Please reduce the slice size and try again."
)

func newErrorBoundary() *errorBoundary {
	errorBoundary := &errorBoundary{
		client: &http.Client{},
	}
	return errorBoundary
}

func newErrorBoundaryForTest(endpoint string) *errorBoundary {
	errorBoundary := &errorBoundary{
		client:   &http.Client{},
		endpoint: endpoint,
	}
	return errorBoundary
}

func (e *errorBoundary) logException(exception error) error {
	var exceptionString string
	if exception == nil {
		exceptionString = "Unknown"
	} else {
		exceptionString = exception.Error()
	}
	var stack []byte
	runtime.Stack(stack, false)
	body := &logExceptionRequestBody{
		Exception: exceptionString,
		Info:      string(stack),
	}
	bodyString, err := json.Marshal(body)
	if err != nil {
		return err
	}
	metadata := getStatsigMetadata()

	req, err := http.NewRequest("POST", e.endpoint, bytes.NewBuffer(bodyString))
	if err != nil {
		return err
	}
	client := http.Client{}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(time.Now().Unix()*1000, 10))
	req.Header.Add("STATSIG-SDK-TYPE", metadata.SDKType)
	req.Header.Add("STATSIG-SDK-VERSION", metadata.SDKVersion)

	var response logExceptionResponse
	httpRes, err := client.Do(req)
	if err != nil {
		return err
	}
	err = json.NewDecoder(httpRes.Body).Decode(&response)
	if err != nil {
		return err
	}
	if !response.Success {
		return errors.New("Log exception not successful")
	}

	return nil
}
