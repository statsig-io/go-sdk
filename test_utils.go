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
	"time"
)

type events []map[string]interface{}

type testServerOptions struct {
	status          int
	onLogEvent      func(events []map[string]interface{})
	onDCS           func()
	onGetIDLists    func()
	withSampling    bool
	isLayerExposure bool
	uaBasedRules    bool
	useCurrentTime  bool
	noUpdateOnSync  bool
}

func getTestServer(opts testServerOptions) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("x-statsig-region", "az-westus-2")
		var status int
		if opts.status == 0 {
			status = http.StatusOK
		} else {
			status = opts.status
		}
		res.WriteHeader(status)
		if status < 200 || status >= 300 {
			return
		}
		if strings.Contains(req.URL.Path, "download_config_specs") {
			dcsFile := "download_config_specs.json"
			if opts.withSampling {
				dcsFile = "download_config_specs_with_diagnostics_sampling.json"
			}
			if opts.isLayerExposure {
				dcsFile = "layer_exposure_download_config_specs.json"
			}
			if opts.uaBasedRules {
				dcsFile = "download_config_specs_ua_gates.json"
			}
			bytes, _ := os.ReadFile(dcsFile)

			if opts.useCurrentTime {
				var configData map[string]interface{}
				if err := json.Unmarshal(bytes, &configData); err == nil {
					configData["time"] = time.Now().UnixNano() / int64(time.Millisecond)
					if updatedBytes, err := json.Marshal(configData); err == nil {
						bytes = updatedBytes
					}
				}
			}

			if opts.noUpdateOnSync {
				sinceTime := req.URL.Query().Get("sinceTime")
				if sinceTime == "" {
					sinceTime = "0"
				}
				if opts.noUpdateOnSync && sinceTime != "0" {
					var configData map[string]interface{}
					if err := json.Unmarshal(bytes, &configData); err == nil {
						configData["has_updates"] = false
						if updatedBytes, err := json.Marshal(configData); err == nil {
							bytes = updatedBytes
						}
					}
				}
			}

			_, _ = res.Write(bytes)
			if opts.onDCS != nil {
				opts.onDCS()
			}
		} else if strings.Contains(req.URL.Path, "log_event") {
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
