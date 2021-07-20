package net

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"statsig/pkg/types"
)

type statsigMetadata struct {
	SDKType    string `json:"sdkType"`
	SDKVersion string `json:"sdkVersion"`
}

type gateResponse struct {
	Name   string `json:"name"`
	Value  bool   `json:"value"`
	RuleID string `json:"rule_id"`
}

type configResponse struct {
	Name   string                 `json:"name"`
	Value  map[string]interface{} `json:"value"`
	RuleID string                 `json:"rule_id"`
}

type checkGateInput struct {
	GateName        string            `json:"gateName"`
	User            types.StatsigUser `json:"user"`
	StatsigMetadata statsigMetadata   `json:"statsigMetadata"`
}

type getConfigInput struct {
	ConfigName      string            `json:"configName"`
	User            types.StatsigUser `json:"user"`
	StatsigMetadata statsigMetadata   `json:"statsigMetadata"`
}

type logEventInput struct {
	Events          []types.StatsigEvent `json:"events"`
	StatsigMetadata statsigMetadata      `json:"statsigMetadata"`
}

type logEventResponse struct{}

type Net struct {
	api      string
	metadata statsigMetadata
	sdkKey   string
	client   *http.Client
}

func New(secret string, api string) *Net {
	return &Net{
		api:      api,
		metadata: statsigMetadata{SDKType: "go-sdk", SDKVersion: "0.0.1"},
		sdkKey:   secret,
		client:   &http.Client{},
	}
}

func (n *Net) CheckGate(user types.StatsigUser, gateName string) bool {
	input := &checkGateInput{
		GateName:        gateName,
		User:            user,
		StatsigMetadata: n.metadata,
	}
	var gateResponse gateResponse
	err := postRequest(n, "check_gate", input, &gateResponse)
	if err != nil {
		return false
	}
	return gateResponse.Value
}

func (n *Net) GetConfig(user types.StatsigUser, configName string) *types.DynamicConfig {
	input := &getConfigInput{
		ConfigName:      configName,
		User:            user,
		StatsigMetadata: n.metadata,
	}
	var configResponse configResponse
	postRequest(n, "get_config", input, &configResponse)

	return types.NewConfig(configResponse.Name, configResponse.Value, configResponse.RuleID)
}

func (n *Net) LogEvents(events []types.StatsigEvent) {
	input := &logEventInput{
		Events:          events,
		StatsigMetadata: n.metadata,
	}
	var res logEventResponse
	postRequest(n, "log_event", input, &res)
}

func postRequest(
	n *Net,
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
	var response *http.Response
	response, err = n.client.Do(req)
	if err != nil {
		return err
	}
	statusOK := response.StatusCode >= 200 && response.StatusCode < 300
	if !statusOK {
		return errors.New(fmt.Sprintf("http response error code: %d", response.StatusCode))
	}
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&out)
	return err
}
