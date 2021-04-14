package statsig

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type StatsigUser struct {
    UserID   string `json:"userID"`
    Email string `json:"email"`
    IpAddress string `json:"ip"`
    UserAgent string `json:"userAgent"`
    Country string `json:"country"`
    Locale string `json:"locale"`
    ClientVersion string `json:"clientVersion"`
    Custom string `json:"custom"`
}

type StatsigMetadata struct {
    SdkType string `json:"sdkType"`
    SdkVersion string `json:"sdkVersion"`
}

type GateResponse struct {
    Name string `json:"name"`
    Value bool `json:"value"`
}

type ConfigResponse struct {
    Name string `json:"name"`
    Value map[string]interface{} `json:"value"`
    Group string `json:"group"`
}

type CheckGateInput struct {
    GateName string `json:"gateName"`
    User StatsigUser `json:"user"`
    StatsigMetadata StatsigMetadata `json:"statsigMetadata"`
}

type GetConfigInput struct {
    ConfigName string `json:"configName"`
    User StatsigUser `json:"user"`
    StatsigMetadata StatsigMetadata `json:"statsigMetadata"`
}

var sdkKey string
var sdkMetadata *StatsigMetadata
var client *http.Client

func init() {
    client = &http.Client{}
}

func Initialize(secretKey string) {
    sdkKey = secretKey
    sdkMetadata = &StatsigMetadata{SdkType: "go-sdk", SdkVersion: "0.0.1"}
}

func CheckGate(user StatsigUser, gateName string) bool {    
    input := &CheckGateInput{GateName: gateName, User: user, StatsigMetadata: *sdkMetadata}
    jsonStr, _ := json.Marshal(input)
    serverResponse := postRequest("check_gate", jsonStr)

    // TODO abstract json parsing and handle errors
    decoder := json.NewDecoder(serverResponse.Body)
    var gateResponse GateResponse
    decoder.Decode(&gateResponse)
    // TODO handle errors

    return gateResponse.Value
}

func GetConfig(user StatsigUser, configName string) map[string]interface{} {
    input := &GetConfigInput{ConfigName: configName, User: user, StatsigMetadata: *sdkMetadata}
    jsonStr, _ := json.Marshal(input)
    serverResponse := postRequest("get_config", jsonStr)

    decoder := json.NewDecoder(serverResponse.Body)
    var configResponse ConfigResponse
    decoder.Decode(&configResponse)

    return configResponse.Value
}

func postRequest(endpoint string, body []byte) *http.Response {
    req, _ := http.NewRequest("POST", "https://api.statsig.com/v1/" + endpoint, bytes.NewBuffer(body))
    req.Header.Add("STATSIG-API-KEY", sdkKey)
    req.Header.Set("Content-Type", "application/json")
    http_response, _ := client.Do(req)
    return http_response
}