package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type StatsigMetadata struct {
	SDKType    string `json:"sdkType"`
	SDKVersion string `json:"sdkVersion"`
}

type Net struct {
	api      string
	metadata StatsigMetadata
	sdkKey   string
	client   *http.Client
}

func New(secret string, api string) *Net {
	return &Net{
		api:      api,
		metadata: StatsigMetadata{SDKType: "go-sdk", SDKVersion: "0.0.1"},
		sdkKey:   secret,
		client:   &http.Client{},
	}
}

func (n *Net) GetStatsigMetadata() StatsigMetadata {
	return n.metadata
}

func (n *Net) PostRequest(
	endpoint string,
	in interface{},
	out interface{},
) error {
	jsonStr, err := json.Marshal(in)
	if err != nil {
		return err
	}

	var req *http.Request
	req, err = http.NewRequest("POST", n.api+endpoint, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Add("STATSIG-API-KEY", n.sdkKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(time.Now().Unix()*1000, 10))
	var response *http.Response
	response, err = n.client.Do(req)
	if err != nil {
		return err
	}
	statusOK := response.StatusCode >= 200 && response.StatusCode < 300
	if !statusOK {
		return fmt.Errorf("http response error code: %d", response.StatusCode)
	}
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&out)
	return err
}
