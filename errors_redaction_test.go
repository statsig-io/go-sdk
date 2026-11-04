package statsig

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type redactionRoundTripper func(*http.Request) (*http.Response, error)

func (f redactionRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type redactionFailReader struct{ err error }

func (r redactionFailReader) Read([]byte) (int, error) { return 0, r.err }
func (r redactionFailReader) Close() error             { return nil }

func TestTransportErrorRedactsCompleteMessage(t *testing.T) {
	const key = "secret-TestKey123"
	sentinel := errors.New("failure with " + key)
	tests := []struct {
		name     string
		metadata *RequestMetadata
		err      error
		want     string
	}{
		{"metadata and cause", &RequestMetadata{Endpoint: "/download_config_specs/" + key + ".json", Retries: 2}, sentinel, "Failed request to /download_config_specs/secret-****.json after 2 retries: failure with secret-****"},
		{"no metadata", nil, &url.Error{Op: "Get", URL: "https://example.com/" + key, Err: sentinel}, "Get \"https://example.com/secret-****\": failure with secret-****"},
		{"no secret", &RequestMetadata{Endpoint: "/log_event"}, errors.New("connection failed"), "Failed request to /log_event after 0 retries: connection failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &TransportError{RequestMetadata: tt.metadata, Err: tt.err}
			if got := err.Error(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if !errors.Is(err, ErrNetworkRequest) || !errors.Is(err, tt.err) {
				t.Error("error identity changed")
			}
			if tt.name == "no metadata" {
				var urlErr *url.Error
				if !errors.As(err, &urlErr) || urlErr != tt.err {
					t.Error("url.Error is not preserved")
				}
			}
		})
	}
}

func TestDownloadConfigSpecsRedactsTransportFailures(t *testing.T) {
	const key = "secret-TestKey123"
	failure := errors.New("read failure")
	tests := []struct {
		name      string
		roundTrip redactionRoundTripper
		metadata  bool
	}{
		{"no response", func(*http.Request) (*http.Response, error) { return nil, failure }, false},
		{"body read", func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: redactionFailReader{failure}, Request: req}, nil
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := newTransport(key, &Options{HTTPClient: &http.Client{Transport: tt.roundTrip}})
			var out interface{}
			_, err := n.download_config_specs(0, &out, nil, nil)
			if err == nil {
				t.Fatal("expected DCS failure")
			}
			if strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "secret-****") {
				t.Errorf("unexpected error: %s", err)
			}
			var transportErr *TransportError
			if !errors.As(err, &transportErr) || (transportErr.RequestMetadata != nil) != tt.metadata {
				t.Fatalf("unexpected transport metadata: %#v", err)
			}
			if !errors.Is(err, failure) {
				t.Error("underlying failure was not preserved")
			}
			if !tt.metadata {
				var urlErr *url.Error
				if !errors.As(err, &urlErr) {
					t.Error("underlying URL error was not preserved")
				}
			}
		})
	}
}

func TestDownloadConfigSpecsRequestBuildErrorLoggerRedactsCallback(t *testing.T) {
	const key = "secret-TestKey123"
	previous := Logger()
	t.Cleanup(func() { global.mu.Lock(); global.logger = previous; global.mu.Unlock() })
	var callbackMsg string
	var callbackErr error
	InitializeGlobalOutputLogger(OutputLoggerOptions{LogCallback: func(msg string, err error) { callbackMsg, callbackErr = msg, err }}, nil)
	n := newTransport(key, &Options{APIOverrides: APIOverrides{DownloadConfigSpecs: "http://[::1"}})
	var out interface{}
	_, err := n.download_config_specs(0, &out, nil, nil)
	if err == nil || callbackErr == nil {
		t.Fatal("expected returned and callback errors")
	}
	for name, value := range map[string]string{"returned error": err.Error(), "callback message": callbackMsg, "callback error": callbackErr.Error()} {
		if strings.Contains(value, key) {
			t.Errorf("%s leaks secret: %s", name, value)
		}
	}
	if !strings.Contains(callbackErr.Error(), "secret-****") {
		t.Errorf("callback error missing redaction: %s", callbackErr)
	}
	if !strings.Contains(fmt.Sprint(err), "secret-****") {
		t.Errorf("returned error missing redaction: %s", err)
	}
}

var _ io.ReadCloser = redactionFailReader{}
