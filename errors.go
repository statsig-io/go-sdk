package statsig

import (
	"errors"
	"fmt"
)

// Error Variables
type StatsigError error

var (
	ErrNetworkRequest StatsigError = errors.New("failed network request")
	ErrFailedLogEvent StatsigError = errors.New("failed to log events")
	ErrDataAdapter    StatsigError = errors.New("failed data adapter")
)

type RequestMetadata struct {
	StatusCode int
	Endpoint   string
	Retries    int
}

type TransportError struct {
	RequestMetadata *RequestMetadata
	Err             error
}

func (e *TransportError) Error() string {
	if e.RequestMetadata != nil {
		return fmt.Sprintf("Failed request to %s after %d retries: %s", e.RequestMetadata.Endpoint, e.RequestMetadata.Retries, e.Err.Error())
	} else {
		return e.Err.Error()
	}
}

func (e *TransportError) Unwrap() error { return e.Err }

func (e *TransportError) Is(target error) bool { return target == ErrNetworkRequest }

type LogEventError struct {
	Err    error
	Events int
}

func (e *LogEventError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Failed to log %d events: %s", e.Events, e.Err.Error())
	} else {
		return fmt.Sprintf("Failed to log %d events", e.Events)
	}
}

func (e *LogEventError) Unwrap() error { return e.Err }

func (e *LogEventError) Is(target error) bool { return target == ErrFailedLogEvent }

type DataAdapterError struct {
	Err    error
	Method string
}

func (e *DataAdapterError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Error calling data adapter %s: %s", e.Method, e.Err.Error())
	} else {
		return fmt.Sprintf("Error calling data adapter %s", e.Method)
	}
}

func (e *DataAdapterError) Unwrap() error { return e.Err }

func (e *DataAdapterError) Is(target error) bool { return target == ErrDataAdapter }
