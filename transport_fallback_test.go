package statsig

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fallbackTestCounter struct {
	proxyDCSCount    int
	cdnDCSCount      int
	proxyIDListCount int
	apiIDListCount   int
}

type mockRoundTripper struct {
	counter        *fallbackTestCounter
	proxyServer    *httptest.Server
	fallbackServer *httptest.Server
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "proxy") || req.URL.Host == m.proxyServer.URL[7:] {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			m.counter.proxyDCSCount++
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			m.counter.proxyIDListCount++
		}
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
			Header:     make(http.Header),
		}, nil
	}

	if strings.Contains(req.URL.Host, "statsigcdn.com") || strings.Contains(req.URL.Host, "statsigapi.net") ||
		req.URL.Host == m.fallbackServer.URL[7:] {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			m.counter.cdnDCSCount++
			response := downloadConfigSpecResponse{
				HasUpdates:   true,
				Time:         getUnixMilli(),
				FeatureGates: []configSpec{{Name: "test_gate"}},
			}
			body, _ := json.Marshal(response)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			m.counter.apiIDListCount++
			response := map[string]interface{}{}
			body, _ := json.Marshal(response)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func TestFallbackToStatsigAPI_DownloadConfigSpecs_HTTP500(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: proxyServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err != nil {
		t.Errorf("Expected successful fallback but got error: %v", err)
	}

	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyDCSCount)
	}

	if counter.cdnDCSCount != 1 {
		t.Errorf("Expected 1 CDN request, got %d", counter.cdnDCSCount)
	}

	if len(responseBody.FeatureGates) > 0 && responseBody.FeatureGates[0].Name != "test_gate" {
		t.Errorf("Expected test_gate in response")
	}
}

func TestFallbackToStatsigAPI_DownloadConfigSpecs_NetworkError(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: proxyServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err != nil {
		t.Errorf("Expected successful fallback but got error: %v", err)
	}

	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyDCSCount)
	}

	if counter.cdnDCSCount != 1 {
		t.Errorf("Expected 1 CDN request, got %d", counter.cdnDCSCount)
	}

	if len(responseBody.FeatureGates) > 0 && responseBody.FeatureGates[0].Name != "test_gate" {
		t.Errorf("Expected test_gate in response")
	}
}

func TestFallbackToStatsigAPI_GetIDLists_HTTP500(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			GetIDLists: proxyServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody map[string]interface{}
	_, err := transport.get_id_lists(&responseBody, nil)

	if err != nil {
		t.Errorf("Expected successful fallback but got error: %v", err)
	}

	if counter.proxyIDListCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyIDListCount)
	}

	if counter.apiIDListCount != 1 {
		t.Errorf("Expected 1 API request, got %d", counter.apiIDListCount)
	}
}

func TestFallbackToStatsigAPI_Disabled(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: proxyServer.URL,
		},
		FallbackToStatsigAPI: false,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err == nil {
		t.Errorf("Expected error when fallback is disabled but got nil")
	}

	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyDCSCount)
	}

	if counter.cdnDCSCount != 0 {
		t.Errorf("Expected 0 CDN requests when fallback disabled, got %d", counter.cdnDCSCount)
	}
}

type mockSuccessfulRoundTripper struct {
	counter *fallbackTestCounter
}

func (m *mockSuccessfulRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "download_config_specs") {
		m.counter.proxyDCSCount++
		response := downloadConfigSpecResponse{
			HasUpdates:   true,
			Time:         getUnixMilli(),
			FeatureGates: []configSpec{{Name: "proxy_gate"}},
		}
		body, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func TestFallbackToStatsigAPI_SuccessfulProxy(t *testing.T) {
	counter := &fallbackTestCounter{}

	mockTransport := &mockSuccessfulRoundTripper{
		counter: counter,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: "https://proxy.example.com",
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err != nil {
		t.Errorf("Expected successful request but got error: %v", err)
	}

	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyDCSCount)
	}

	if counter.cdnDCSCount != 0 {
		t.Errorf("Expected 0 CDN requests when proxy succeeds, got %d", counter.cdnDCSCount)
	}

	if len(responseBody.FeatureGates) > 0 && responseBody.FeatureGates[0].Name != "proxy_gate" {
		t.Errorf("Expected proxy_gate in response")
	}
}

type mockNetworkErrorRoundTripper struct {
	counter        *fallbackTestCounter
	proxyServer    *httptest.Server
	fallbackServer *httptest.Server
}

func (m *mockNetworkErrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Return actual network error (not HTTP error) for proxy server
	if strings.Contains(req.URL.Host, "proxy") || req.URL.Host == m.proxyServer.URL[7:] {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			m.counter.proxyDCSCount++
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			m.counter.proxyIDListCount++
		}
		// Return nil response with error to simulate network error (timeout, connection refused, etc.)
		return nil, errors.New("network error: connection timeout")
	}

	// Successfully respond for CDN/API fallback
	if strings.Contains(req.URL.Host, "statsigcdn.com") || strings.Contains(req.URL.Host, "statsigapi.net") ||
		req.URL.Host == m.fallbackServer.URL[7:] {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			m.counter.cdnDCSCount++
			response := downloadConfigSpecResponse{
				HasUpdates:   true,
				Time:         getUnixMilli(),
				FeatureGates: []configSpec{{Name: "cdn_gate"}},
			}
			body, _ := json.Marshal(response)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			m.counter.apiIDListCount++
			response := map[string]interface{}{}
			body, _ := json.Marshal(response)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func TestFallbackToStatsigAPI_DownloadConfigSpecs_NetworkErrorWithNilResponse(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockNetworkErrorRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: proxyServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err != nil {
		t.Errorf("Expected successful fallback but got error: %v", err)
	}

	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyDCSCount)
	}

	if counter.cdnDCSCount != 1 {
		t.Errorf("Expected 1 CDN request after network error, got %d", counter.cdnDCSCount)
	}

	if len(responseBody.FeatureGates) > 0 && responseBody.FeatureGates[0].Name != "cdn_gate" {
		t.Errorf("Expected cdn_gate in response, got %s", responseBody.FeatureGates[0].Name)
	}
}

func TestFallbackToStatsigAPI_GetIDLists_NetworkErrorWithNilResponse(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockNetworkErrorRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			GetIDLists: proxyServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody map[string]interface{}
	_, err := transport.get_id_lists(&responseBody, nil)

	if err != nil {
		t.Errorf("Expected successful fallback but got error: %v", err)
	}

	if counter.proxyIDListCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyIDListCount)
	}

	if counter.apiIDListCount != 1 {
		t.Errorf("Expected 1 API request after network error, got %d", counter.apiIDListCount)
	}
}

func TestFallbackToStatsigAPI_NetworkError_Disabled(t *testing.T) {
	counter := &fallbackTestCounter{}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Proxy server should not be called directly")
	}))
	defer proxyServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called directly")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockNetworkErrorRoundTripper{
		counter:        counter,
		proxyServer:    proxyServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: proxyServer.URL,
		},
		FallbackToStatsigAPI: false,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err == nil {
		t.Errorf("Expected error when fallback is disabled but got nil")
	}

	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected 1 proxy request, got %d", counter.proxyDCSCount)
	}

	if counter.cdnDCSCount != 0 {
		t.Errorf("Expected 0 CDN requests when fallback disabled, got %d", counter.cdnDCSCount)
	}
}

type mockCustomAPISuccessRoundTripper struct {
	counter        *fallbackTestCounter
	customAPIServer *httptest.Server
	fallbackServer  *httptest.Server
}

func (m *mockCustomAPISuccessRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Custom API (SFP) succeeds immediately
	if strings.Contains(req.URL.Host, "custom") || req.URL.Host == m.customAPIServer.URL[7:] {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			m.counter.proxyDCSCount++
			response := downloadConfigSpecResponse{
				HasUpdates:   true,
				Time:         getUnixMilli(),
				FeatureGates: []configSpec{{Name: "custom_api_gate"}},
			}
			body, _ := json.Marshal(response)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			m.counter.proxyIDListCount++
			response := map[string]interface{}{}
			body, _ := json.Marshal(response)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}
	}

	// Fallback API should never be called if custom API succeeds
	if strings.Contains(req.URL.Host, "statsigcdn.com") || strings.Contains(req.URL.Host, "statsigapi.net") ||
		req.URL.Host == m.fallbackServer.URL[7:] {
		if strings.Contains(req.URL.Path, "download_config_specs") {
			m.counter.cdnDCSCount++
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			m.counter.apiIDListCount++
		}
		// This should not happen if custom API succeeds
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("Fallback should not be called")),
			Header:     make(http.Header),
		}, nil
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func TestFallbackToStatsigAPI_CustomAPISucceeds_NoRetry(t *testing.T) {
	counter := &fallbackTestCounter{}

	customAPIServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Custom API succeeds
		response := downloadConfigSpecResponse{
			HasUpdates:   true,
			Time:         getUnixMilli(),
			FeatureGates: []configSpec{{Name: "custom_api_gate"}},
		}
		body, _ := json.Marshal(response)
		res.WriteHeader(200)
		_, _ = res.Write(body)
	}))
	defer customAPIServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called when custom API succeeds")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockCustomAPISuccessRoundTripper{
		counter:        counter,
		customAPIServer: customAPIServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: customAPIServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err != nil {
		t.Errorf("Expected successful request but got error: %v", err)
	}

	// Should only call custom API once, no retries
	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected exactly 1 custom API request, got %d", counter.proxyDCSCount)
	}

	// Should never call fallback API when custom API succeeds
	if counter.cdnDCSCount != 0 {
		t.Errorf("Expected 0 fallback CDN requests when custom API succeeds, got %d", counter.cdnDCSCount)
	}

	if len(responseBody.FeatureGates) > 0 && responseBody.FeatureGates[0].Name != "custom_api_gate" {
		t.Errorf("Expected custom_api_gate in response, got %s", responseBody.FeatureGates[0].Name)
	}
}

func TestFallbackToStatsigAPI_CustomAPISucceeds_GetIDLists_NoRetry(t *testing.T) {
	counter := &fallbackTestCounter{}

	customAPIServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Custom API succeeds
		response := map[string]interface{}{}
		body, _ := json.Marshal(response)
		res.WriteHeader(200)
		_, _ = res.Write(body)
	}))
	defer customAPIServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called when custom API succeeds")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockCustomAPISuccessRoundTripper{
		counter:        counter,
		customAPIServer: customAPIServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			GetIDLists: customAPIServer.URL,
		},
		FallbackToStatsigAPI: true,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody map[string]interface{}
	_, err := transport.get_id_lists(&responseBody, nil)

	if err != nil {
		t.Errorf("Expected successful request but got error: %v", err)
	}

	// Should only call custom API once, no retries
	if counter.proxyIDListCount != 1 {
		t.Errorf("Expected exactly 1 custom API request, got %d", counter.proxyIDListCount)
	}

	// Should never call fallback API when custom API succeeds
	if counter.apiIDListCount != 0 {
		t.Errorf("Expected 0 fallback API requests when custom API succeeds, got %d", counter.apiIDListCount)
	}
}

func TestFallbackToStatsigAPI_CustomAPISucceeds_FallbackDisabled_NoRetry(t *testing.T) {
	counter := &fallbackTestCounter{}

	customAPIServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Custom API succeeds
		response := downloadConfigSpecResponse{
			HasUpdates:   true,
			Time:         getUnixMilli(),
			FeatureGates: []configSpec{{Name: "custom_api_gate"}},
		}
		body, _ := json.Marshal(response)
		res.WriteHeader(200)
		_, _ = res.Write(body)
	}))
	defer customAPIServer.Close()

	fallbackServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		t.Error("Fallback server should not be called when fallback is disabled")
	}))
	defer fallbackServer.Close()

	mockTransport := &mockCustomAPISuccessRoundTripper{
		counter:        counter,
		customAPIServer: customAPIServer,
		fallbackServer: fallbackServer,
	}

	opt := &Options{
		APIOverrides: APIOverrides{
			DownloadConfigSpecs: customAPIServer.URL,
		},
		FallbackToStatsigAPI: false,
		Transport:            mockTransport,
	}

	transport := newTransport("secret-123", opt)

	var responseBody downloadConfigSpecResponse
	_, err := transport.download_config_specs(0, &responseBody, nil, nil)

	if err != nil {
		t.Errorf("Expected successful request but got error: %v", err)
	}

	// Should only call custom API once
	if counter.proxyDCSCount != 1 {
		t.Errorf("Expected exactly 1 custom API request, got %d", counter.proxyDCSCount)
	}

	// Should never call fallback API (fallback is disabled)
	if counter.cdnDCSCount != 0 {
		t.Errorf("Expected 0 fallback CDN requests when fallback is disabled, got %d", counter.cdnDCSCount)
	}
}
