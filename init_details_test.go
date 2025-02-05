package statsig

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInitDetails(t *testing.T) {
	configSpecBytes, _ := os.ReadFile("download_config_specs.json")

	t.Run("Network - success", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()
			res.WriteHeader(http.StatusOK)
			if strings.Contains(req.URL.Path, "download_config_specs") {
				_, _ = res.Write(configSpecBytes)
			}
		}))
		defer testServer.Close()

		options := &Options{
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if !details.Success {
			t.Errorf("Expected initalize success to be true")
		}
		if details.Error != nil {
			t.Errorf("Expected initalize to have no errors")
		}
		if details.Source != SourceNetwork {
			t.Errorf("Expected initalize source to be Network")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Network - failure", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "download_config_specs") {
				res.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer testServer.Close()

		options := &Options{
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if details.Success {
			t.Errorf("Expected initalize success to be false")
		}
		if !errors.Is(details.Error, ErrNetworkRequest) {
			t.Errorf("Expected initalize to have network error")
		}
		if details.Source != SourceUninitialized {
			t.Errorf("Expected initalize source to be Uninitialized")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Network - timeout", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "download_config_specs") {
				time.Sleep(100 * time.Millisecond)
			}
		}))
		defer testServer.Close()

		options := &Options{
			API:                  testServer.URL,
			InitTimeout:          100 * time.Millisecond,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if details.Success {
			t.Errorf("Expected initalize success to be false")
		}
		if details.Error.Error() != "timed out" {
			t.Errorf("Expected initalize to have timeout error")
		}
		if details.Source != SourceUninitialized {
			t.Errorf("Expected initalize source to be Uninitialized")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Bootstrap - success", func(t *testing.T) {
		options := &Options{
			BootstrapValues:      string(configSpecBytes[:]),
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if !details.Success {
			t.Errorf("Expected initalize success to be true")
		}
		if details.Error != nil {
			t.Errorf("Expected initalize to have no errors")
		}
		if details.Source != SourceBootstrap {
			t.Errorf("Expected initalize source to be Bootstrap")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Bootstrap - failure (fallback to network success)", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()
			res.WriteHeader(http.StatusOK)
			if strings.Contains(req.URL.Path, "download_config_specs") {
				_, _ = res.Write(configSpecBytes)
			}
		}))
		defer testServer.Close()

		options := &Options{
			BootstrapValues:      "<invalid>",
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if !details.Success {
			t.Errorf("Expected initalize success to be true")
		}
		if details.Error.Error() != "Failed to parse bootstrap values" {
			t.Errorf("Expected initalize to have bootstrap parsing error")
		}
		if details.Source != SourceNetwork {
			t.Errorf("Expected initalize source to be Network")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Bootstrap - failure (fallback to network failure)", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "download_config_specs") {
				res.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer testServer.Close()

		options := &Options{
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if details.Success {
			t.Errorf("Expected initalize success to be false")
		}
		if !errors.Is(details.Error, ErrNetworkRequest) {
			t.Errorf("Expected initalize to have network error")
		}
		if details.Source != SourceUninitialized {
			t.Errorf("Expected initalize source to be Uninitialized")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Data Adapter - success", func(t *testing.T) {
		dataAdapter := dataAdapterExample{store: make(map[string]string)}
		dataAdapter.Initialize()
		defer dataAdapter.Shutdown()
		dataAdapter.Set(CONFIG_SPECS_KEY, string(configSpecBytes))
		options := &Options{
			DataAdapter:          &dataAdapter,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if !details.Success {
			t.Errorf("Expected initalize success to be true")
		}
		if details.Error != nil {
			t.Errorf("Expected initalize to have no errors")
		}
		if details.Source != SourceDataAdapter {
			t.Errorf("Expected initalize source to be DataAdapter")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Data Adapter - failure (fallback to network success)", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()
			res.WriteHeader(http.StatusOK)
			if strings.Contains(req.URL.Path, "download_config_specs") {
				_, _ = res.Write(configSpecBytes)
			}
		}))
		defer testServer.Close()

		options := &Options{
			DataAdapter:          &brokenDataAdapterExample{},
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if !details.Success {
			t.Errorf("Expected initalize success to be true")
		}
		if !errors.Is(details.Error, ErrDataAdapter) {
			t.Errorf("Expected initalize to have data adapter error")
		}
		if details.Source != SourceNetwork {
			t.Errorf("Expected initalize source to be Network")
		}
		ShutdownAndDangerouslyClearInstance()
	})

	t.Run("Data Adapter - failure (fallback to network failure)", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "download_config_specs") {
				res.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer testServer.Close()

		options := &Options{
			DataAdapter:          &brokenDataAdapterExample{},
			API:                  testServer.URL,
			OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
			StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
		}
		start := time.Now()
		details := InitializeWithOptions("secret-key", options)
		elapsed := time.Since(start)
		if details.Duration == 0 || details.Duration > elapsed {
			t.Errorf("Expected initalize duration in details")
		}
		if details.Success {
			t.Errorf("Expected initalize success to be false")
		}
		if !errors.Is(details.Error, ErrNetworkRequest) {
			t.Errorf("Expected initalize to have network error")
		}
		if details.Source != SourceUninitialized {
			t.Errorf("Expected initalize source to be Uninitialized")
		}
		ShutdownAndDangerouslyClearInstance()
	})
}
