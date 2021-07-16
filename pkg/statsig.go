package statsig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"statsig/internal/evaluation"
	"statsig/pkg/types"
)

type statsigMetadata struct {
	SDKType    string `json:"sdkType"`
	SDKVersion string `json:"sdkVersion"`
}

type gateResponse struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

type configResponse struct {
	Name  string                 `json:"name"`
	Value map[string]interface{} `json:"value"`
	Group string                 `json:"group"`
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

type Statsig struct {
	// TODO: fill this
}

// TODO: I *think* these need to live inside some StatsigClient struct
var sdkKey string
var sdkMetadata *statsigMetadata
var client *http.Client
var evaluator *evaluation.Evaluator

func init() {
	client = &http.Client{}
}

func Initialize(secretKey string) *Statsig {
	sdkKey = secretKey
	sdkMetadata = &statsigMetadata{SDKType: "go-sdk", SDKVersion: "0.0.1"}
	evaluator = evaluation.New(secretKey)
	return new(Statsig)
}

func CheckGate(user types.StatsigUser, gateName string) bool {
	input := &checkGateInput{GateName: gateName, User: user, StatsigMetadata: *sdkMetadata}
	jsonStr, _ := json.Marshal(input)
	serverResponse := postRequest("check_gate", jsonStr)

	// TODO abstract json parsing and handle errors
	decoder := json.NewDecoder(serverResponse.Body)
	var gateResponse gateResponse
	decoder.Decode(&gateResponse)
	// TODO handle errors

	return gateResponse.Value
}

func GetConfig(user types.StatsigUser, configName string) map[string]interface{} {
	input := &getConfigInput{ConfigName: configName, User: user, StatsigMetadata: *sdkMetadata}
	jsonStr, _ := json.Marshal(input)
	serverResponse := postRequest("get_config", jsonStr)

	decoder := json.NewDecoder(serverResponse.Body)
	var configResponse configResponse
	decoder.Decode(&configResponse)

	return configResponse.Value
}

func postRequest(endpoint string, body []byte) *http.Response {
	req, _ := http.NewRequest("POST", "https://api.statsig.com/v1/"+endpoint, bytes.NewBuffer(body))
	req.Header.Add("STATSIG-API-KEY", sdkKey)
	req.Header.Set("Content-Type", "application/json")
	http_response, _ := client.Do(req)
	return http_response
}
