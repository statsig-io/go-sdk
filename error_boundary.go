package statsig

import (
	"runtime"
)

type errorBoundary struct {
	transport *transport
	endpoint  string `default:"https://statsigapi.net/v1/sdk_exception"`
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

func newErrorBoundary(transport *transport) *errorBoundary {
	errorBoundary := &errorBoundary{
		transport: transport,
	}
	return errorBoundary
}

func (e *errorBoundary) logException(exception error) bool {
	if exception == nil {
		return false
	}
	var stack []byte
	runtime.Stack(stack, false)
	body := &logExceptionRequestBody{
		Exception: exception.Error(),
		Info:      string(stack),
	}
	var response logExceptionResponse
	err := e.transport.postRequest(e.endpoint, body, &response)
	return err == nil && response.Success
}
