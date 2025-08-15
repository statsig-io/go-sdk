package statsig

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogEventErrors(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusBadRequest)
	}))
	defer testServer.Close()

	errs := make([]error, 0)
	opts := &Options{
		API: testServer.URL,
		StatsigLoggerOptions: StatsigLoggerOptions{
			DisableInitDiagnostics: true,
			DisableSyncDiagnostics: true,
			DisableApiDiagnostics:  true,
		},
		OutputLoggerOptions: OutputLoggerOptions{
			EnableDebug: true,
			LogCallback: func(message string, err error) {
				errs = append(errs, err)
			},
		},
	}
	diagnostics := newDiagnostics(opts)
	transport := newTransport("secret", opts)
	errorBoundary := newErrorBoundary("secret", opts, nil)
	sdkConfigs := newSDKConfigs()
	logger := newLogger(transport, opts, diagnostics, errorBoundary, sdkConfigs)

	user := User{
		UserID: "123",
	}
	event := Event{
		EventName: "test_event",
		User:      user,
		Value:     "3",
	}

	stderrLogs := swallow_stderr(func() {
		logger.logCustom(event)
		logger.flush(true)
	})

	if stderrLogs == "" {
		t.Errorf("Expected output to stderr")
	}

	InitializeGlobalOutputLogger(opts.OutputLoggerOptions, nil)
	logger.logCustom(event)
	logger.flush(true)

	if len(errs) == 0 {
		t.Errorf("Expected output to callback")
	}

	if !errors.Is(errs[0], ErrFailedLogEvent) {
		t.Errorf("Expected error to match ErrFailedLogEvent")
	}

	if errs[0].Error() != "Failed to log 1 events: Failed request to /log_event after 0 retries: 400 Bad Request" {
		t.Errorf("Expected error message")
	}
}
