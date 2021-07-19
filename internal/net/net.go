package net

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"encoding/json"
	"net/http"
	"statsig/pkg/types"
)

type statsigMetadata struct {
	SDKType    string `json:"sdkType"`
	SDKVersion string `json:"sdkVersion"`
}

type gateResponse struct {
	Name	string `json:"name"`
	Value	bool   `json:"value"`
	RuleID	bool   `json:"rule_id"`
}

type configResponse struct {
	Name	string                 `json:"name"`
	Value	map[string]interface{} `json:"value"`
	RuleID	string                 `json:"rule_id"`
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

type Net struct {
	api			string
	metadata 	statsigMetadata
	sdkKey   	string
	client 		*http.Client
}

func New(secret string, api string) *Net {
	
	return &Net {
		api: api,
		metadata: statsigMetadata{SDKType: "go-sdk", SDKVersion: "0.0.1"},
		sdkKey: secret,
		client: &http.Client{},
	}
}

func (n *Net) CheckGate(user types.StatsigUser, gateName string) bool {
	input := &checkGateInput{
		GateName: gateName,
		User: user,
		StatsigMetadata: n.metadata,
	}
	var gateResponse gateResponse
	err := postRequest(n, "check_gate", input, &gateResponse)
	if (err != nil) {
		log.Fatal(err)
		return false
	}
	fmt.Sprintf(gateResponse.Name)
	return gateResponse.Value
}

func (n *Net) GetConfig(user types.StatsigUser, configName string) map[string]interface{} {
	input := &getConfigInput{
		ConfigName: configName,
		User: user,
		StatsigMetadata: n.metadata,
	}
	var configResponse configResponse
	postRequest(n, "get_config", input, &configResponse)

	return configResponse.Value
}

func postRequest(
	n *Net,
	endpoint string,
	in interface{},
	out interface{},
) error {
	jsonStr, err := json.Marshal(in)
	if (err != nil) {
		return err
	}
	var req *http.Request
	req, err = http.NewRequest("POST", n.api, bytes.NewBuffer(jsonStr))
	if (err != nil) {
		return err
	}
	req.Header.Add("STATSIG-API-KEY", n.sdkKey)
	fmt.Println(n.api)
	fmt.Println(n.sdkKey)
	req.Header.Set("Content-Type", "application/json")
	var response *http.Response
	fmt.Println(formatRequest(req))
	response, err = n.client.Do(req)
	if (err != nil) {
		return err
	}
	statusOK := response.StatusCode >= 200 && response.StatusCode < 300
	if (!statusOK) {
		return errors.New(fmt.Sprintf("http response error code: %d", response.StatusCode))
	}
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&out)
	return err
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
	  name = strings.ToLower(name)
	  for _, h := range headers {
		request = append(request, fmt.Sprintf("%v: %v", name, h))
	  }
	}
	
	// If this is a POST, add post data
	if r.Method == "POST" {
	   r.ParseForm()
	   request = append(request, "\n")
	   request = append(request, r.Form.Encode())
	} 
	 // Return the request as a string
	 return strings.Join(request, "\n")
   }