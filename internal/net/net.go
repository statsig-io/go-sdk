package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const backoffMultiplier = 10

const (
	MaxRetries = 5
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

func New(secret string, api string, sdkType string, sdkVersion string) *Net {
	if api == "" {
		api = "https://api.statsig.com/v1"
	}
	if strings.HasSuffix(api, "/") {
		api = api[:len(api)-1]
	}

	if sdkType == "" {
		sdkType = "go-sdk"
	}
	if sdkVersion == "" {
		sdkVersion = "0.2.1"
	}

	return &Net{
		api:      api,
		metadata: StatsigMetadata{SDKType: sdkType, SDKVersion: sdkVersion},
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
	return n.postRequestInternal(endpoint, in, out, 0, 0)
}

func (n *Net) RetryablePostRequest(
	endpoint string,
	in interface{},
	out interface{},
	retries int,
) error {
	return n.postRequestInternal(endpoint, in, out, retries, 1)
}

func (n *Net) postRequestInternal(
	endpoint string,
	in interface{},
	out interface{},
	retries int,
	backoff int,
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
		if retries > 0 {
			time.Sleep(time.Duration(backoff) * time.Second)
			return n.postRequestInternal(endpoint, in, out, retries-1, backoff*backoffMultiplier)
		}
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		err := json.NewDecoder(response.Body).Decode(&out)
		return err
	} else if retries > 0 {
		retry := retryCodes()
		if retry(response.StatusCode) {
			time.Sleep(time.Duration(backoff) * time.Second)
			return n.postRequestInternal(endpoint, in, out, retries-1, backoff*backoffMultiplier)
		}
	}
	return fmt.Errorf("http response error code: %d", response.StatusCode)
}

func retryCodes() func(int) bool {
	codes := map[int]bool{
		408: true,
		500: true,
		502: true,
		503: true,
		504: true,
		522: true,
		524: true,
		599: true,
	}
	return func(key int) bool {
		_, ok := codes[key]
		return ok
	}
}
