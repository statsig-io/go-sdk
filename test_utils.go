package statsig

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
)

type events []map[string]interface{}

type testServerOptions struct {
	dcsOnline       bool
	onLogEvent      func(events []map[string]interface{})
	onDCS           func()
	onGetIDLists    func()
	withSampling    bool
	isLayerExposure bool
}

func getTestServer(opts testServerOptions) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("x-statsig-region", "az-westus-2")
		if strings.Contains(req.URL.Path, "download_config_specs") {
			if !opts.dcsOnline {
				res.WriteHeader(http.StatusInternalServerError)
			} else {
				dcsFile := "download_config_specs.json"
				if opts.withSampling {
					dcsFile = "download_config_specs_with_diagnostics_sampling.json"
				}
				if opts.isLayerExposure {
					dcsFile = "layer_exposure_download_config_specs.json"
				}
				bytes, _ := os.ReadFile(dcsFile)
				res.WriteHeader(http.StatusOK)
				_, _ = res.Write(bytes)
			}
			if opts.onDCS != nil {
				opts.onDCS()
			}
		} else if strings.Contains(req.URL.Path, "log_event") {
			res.WriteHeader(http.StatusOK)
			type requestInput struct {
				Events          []map[string]interface{} `json:"events"`
				StatsigMetadata statsigMetadata          `json:"statsigMetadata"`
			}
			input := &requestInput{}
			defer req.Body.Close()
			if req.Header.Get("Content-Encoding") == "gzip" {
				gz, _ := gzip.NewReader(req.Body)
				bodyBytes, _ := io.ReadAll(gz)
				_ = json.Unmarshal(bodyBytes, &input)
				gz.Close()
			} else {
				buf := new(bytes.Buffer)
				_, _ = buf.ReadFrom(req.Body)

				_ = json.Unmarshal(buf.Bytes(), &input)
			}

			if opts.onLogEvent != nil {
				opts.onLogEvent(input.Events)
			}
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			res.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(map[string]map[string]interface{}{
				"my_id_list": {
					"name":         "my_id_list",
					"size":         1,
					"url":          fmt.Sprintf("%s/my_id_list", getTestIDListServer().URL),
					"creationTime": 1,
					"fileID":       "a_file_id",
				},
			})
			_, _ = res.Write(response)
			if opts.onGetIDLists != nil {
				opts.onGetIDLists()
			}
		}
	}))
}

func getTestIDListServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "my_id_list") {
			res.WriteHeader(http.StatusOK)
			response, _ := json.Marshal("+asdfcd")
			_, _ = res.Write(response)
		}
	}))
}

func convertToExposureEvent(eventData map[string]interface{}) Event {
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		fmt.Println("Error marshalling:", err)
		return Event{}
	}
	var event Event
	if err := json.Unmarshal(eventJSON, &event); err != nil {
		fmt.Println("Error unmarshalling:", err)
		return Event{}
	}
	return event
}
