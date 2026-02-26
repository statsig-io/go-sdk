package statsig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type Empty struct{}

type ServerResponse struct {
	Name string `json:"name"`
}

func TestNonRetryable(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			t.Errorf("Expected ‘POST’ request, got '%s'", req.Method)
		}

		res.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()
	in := Empty{}
	var out ServerResponse
	opt := &Options{
		API: testServer.URL,
	}
	n := newTransport("secret-123", opt)
	_, err := n.post("/123", in, &out, RequestOptions{retries: 2}, nil)
	if err == nil {
		t.Errorf("Expected error for network request but got nil")
	}
}

func TestLocalMode(t *testing.T) {
	hit := false
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		hit = true
		res.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()
	in := Empty{}
	var out ServerResponse
	opt := &Options{
		API:       testServer.URL,
		LocalMode: true,
	}
	n := newTransport("secret-123", opt)
	_, err := n.post("/123", in, &out, RequestOptions{retries: 2}, nil)
	if err != nil {
		t.Errorf("Expected no error for network request")
	}
	if hit {
		t.Errorf("Expected transport class not to hit the server")
	}
}

func TestRetries(t *testing.T) {
	tries := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer func() {
			tries = tries + 1
		}()
		switch tries {
		case 0:
			res.WriteHeader(http.StatusInternalServerError)
		case 1:
			output := ServerResponse{
				Name: "test",
			}
			res.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(res).Encode(output)
		}
	}))
	defer func() { testServer.Close() }()
	in := Empty{}
	var out ServerResponse
	opt := &Options{
		API: testServer.URL,
	}
	n := newTransport("secret-123", opt)
	_, err := n.post("/123", in, out, RequestOptions{retries: 2}, nil)
	if err != nil {
		t.Errorf("Expected successful request but got error")
	}
}

func TestProxy(t *testing.T) {
	testServerHit := false
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		testServerHit = true
	}))
	defer testServer.Close()
	in := Empty{}
	var out ServerResponse
	url, _ := url.Parse(testServer.URL)
	opt := &Options{
		Transport: &http.Transport{Proxy: http.ProxyURL(url)},
	}
	n := newTransport("secret-123", opt)
	_, _ = n.post("/123", in, &out, RequestOptions{}, nil)
	if !testServerHit {
		t.Errorf("Expected request to hit proxy server")
	}
}

func TestDefaultNetworkTimeout(t *testing.T) {
	n := newTransport("secret-123", &Options{})
	if n.client.Timeout != defaultTimeout {
		t.Errorf("Expected default timeout %s, got %s", defaultTimeout, n.client.Timeout)
	}
}

func TestCustomNetworkTimeout(t *testing.T) {
	timeout := 5 * time.Second
	n := newTransport("secret-123", &Options{NetworkTimeout: timeout})
	if n.client.Timeout != timeout {
		t.Errorf("Expected timeout %s, got %s", timeout, n.client.Timeout)
	}
}

func TestCustomHTTPClient(t *testing.T) {
	customTransport := &http.Transport{}
	customClient := &http.Client{
		Timeout:   7 * time.Second,
		Transport: customTransport,
	}

	n := newTransport("secret-123", &Options{
		HTTPClient:     customClient,
		NetworkTimeout: time.Second,
		Transport:      &http.Transport{},
	})

	if n.client != customClient {
		t.Errorf("Expected transport to use provided HTTP client")
	}
	if n.client.Timeout != 7*time.Second {
		t.Errorf("Expected provided client timeout to be preserved")
	}
	if n.client.Transport != customTransport {
		t.Errorf("Expected provided client transport to be preserved")
	}
}

func TestNetworkTimeoutAffectsRequests(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		time.Sleep(200 * time.Millisecond)
		res.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(res).Encode(ServerResponse{Name: "slow"})
	}))
	defer testServer.Close()

	n := newTransport("secret-123", &Options{
		API:            testServer.URL,
		NetworkTimeout: 20 * time.Millisecond,
	})

	start := time.Now()
	_, err := n.post("/123", Empty{}, &ServerResponse{}, RequestOptions{}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("Expected request to time out")
	}
	if elapsed >= 150*time.Millisecond {
		t.Errorf("Expected timeout before server response, got %s", elapsed)
	}
}

func TestCustomHTTPClientOverridesNetworkTimeoutForRequests(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		time.Sleep(50 * time.Millisecond)
		res.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(res).Encode(ServerResponse{Name: "ok"})
	}))
	defer testServer.Close()

	n := newTransport("secret-123", &Options{
		API:            testServer.URL,
		NetworkTimeout: 10 * time.Millisecond,
		HTTPClient: &http.Client{
			Timeout: 200 * time.Millisecond,
		},
	})

	var out ServerResponse
	start := time.Now()
	_, err := n.post("/123", Empty{}, &out, RequestOptions{}, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected request to succeed with custom HTTP client, got %v", err)
	}
	if out.Name != "ok" {
		t.Errorf("Expected response body to be decoded")
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("Expected request to wait for server response, got %s", elapsed)
	}
}

func TestDownloadConfigSpecsLogsRequestBuildErrors(t *testing.T) {
	InitializeGlobalOutputLogger(OutputLoggerOptions{}, nil)

	n := newTransport("secret-123", &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: "http://[::1",
		},
	})

	var out downloadConfigSpecResponse
	var err error
	stderrLogs := swallow_stderr(func() {
		_, err = n.download_config_specs(0, &out, nil, nil)
	})

	if err == nil {
		t.Fatalf("Expected request build failure for invalid download_config_specs override")
	}
	if !strings.Contains(stderrLogs, "download_config_specs") {
		t.Errorf("Expected stderr logs to mention download_config_specs, got %q", stderrLogs)
	}
	if !strings.Contains(stderrLogs, "base_api=http://[::1") {
		t.Errorf("Expected stderr logs to mention invalid base API, got %q", stderrLogs)
	}
	if !strings.Contains(stderrLogs, "endpoint=/download_config_specs/secret-****.json") {
		t.Errorf("Expected stderr logs to mention the endpoint path, got %q", stderrLogs)
	}
}
