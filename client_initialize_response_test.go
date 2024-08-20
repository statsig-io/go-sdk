package statsig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"
)

func TestInitializeResponseConsistency(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Disabled until optimization are complete")
	}

	user := User{
		UserID:    "123",
		Email:     "test@statsig.com",
		Country:   "US",
		Custom:    map[string]interface{}{"test": "123"},
		CustomIDs: map[string]string{"stableID": "12345"},
	}

	for _, api := range testAPIs {
		t.Run("Validate consistency for "+api, func(t *testing.T) {
			endpoint := api + "/initialize"
			input := map[string]interface{}{
				"user": user,
				"statsigMetadata": map[string]string{
					"sdkType":   "consistency-test",
					"sessionID": "x123",
				},
			}
			body, _ := json.Marshal(input)
			req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
			if err != nil {
				t.Errorf("Failed to make a request to %s", endpoint)
			}

			clientKey := os.Getenv("test_client_key")
			req.Header.Add("STATSIG-API-KEY", clientKey)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Add("STATSIG-CLIENT-TIME", strconv.FormatInt(getUnixMilli(), 10))
			req.Header.Add("STATSIG-SDK-TYPE", getStatsigMetadata().SDKType)
			req.Header.Add("STATSIG-SDK-VERSION", getStatsigMetadata().SDKVersion)
			req.Header.Set("User-Agent", "")
			client := http.Client{}
			response, err := client.Do(req)
			if err != nil {
				t.Errorf("Failed to get a valid response from %s", endpoint)
			}
			defer response.Body.Close()

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				t.Errorf("Request to %s failed with status %d", endpoint, response.StatusCode)
			}

			actualResponseBody, err := filterHttpResponseAndReadBody(response)
			if err != nil {
				t.Errorf("Error reading %s response body", endpoint)
			}

			InitializeWithOptions(secret, &Options{
				API:                  api,
				OutputLoggerOptions:  getOutputLoggerOptionsForTest(t),
				StatsigLoggerOptions: getStatsigLoggerOptionsForTest(t),
			})
			defer ShutdownAndDangerouslyClearInstance()

			formattedResponse := GetClientInitializeResponse(user)
			filterClientInitializeResponse(&formattedResponse)
			formattedResponseJson, _ := json.Marshal(formattedResponse)

			formattedResponseWithOptions := GetClientInitializeResponseWithOptions(user, &GCIROptions{})
			filterClientInitializeResponse(&formattedResponseWithOptions)
			formattedResponseWithOptionsJson, _ := json.Marshal(formattedResponseWithOptions)

			if string(actualResponseBody) != string(formattedResponseJson) {
				t.Errorf("Inconsistent response from GetClientInitializeResponse vs %s", endpoint)
			}
			if string(actualResponseBody) != string(formattedResponseWithOptionsJson) {
				t.Errorf("Inconsistent response from GetClientInitializeResponseWithOptions vs %s", endpoint)
			}
		})
	}
}

func filterHttpResponseAndReadBody(httpResponse *http.Response) ([]byte, error) {
	var interfaceBody ClientInitializeResponse
	// Initialize nullable fields so that JSON Unmarshal doesn't convert to null
	interfaceBody.FeatureGates = make(map[string]GateInitializeResponse)
	interfaceBody.DynamicConfigs = make(map[string]ConfigInitializeResponse)
	interfaceBody.LayerConfigs = make(map[string]LayerInitializeResponse)
	interfaceBody.SdkParams = make(map[string]string)
	interfaceBody.EvaluatedKeys = make(map[string]interface{})
	err := json.NewDecoder(httpResponse.Body).Decode(&interfaceBody)
	if err != nil {
		return make([]byte, 0), err
	}
	filterClientInitializeResponse(&interfaceBody)
	return json.Marshal(interfaceBody)
}

func filterClientInitializeResponse(clientInitializeResponse *ClientInitializeResponse) {
	for i := range clientInitializeResponse.FeatureGates {
		for j := range clientInitializeResponse.FeatureGates[i].SecondaryExposures {
			clientInitializeResponse.FeatureGates[i].SecondaryExposures[j].Gate = "__REMOVED_FOR_TEST__"
		}
	}
	for i := range clientInitializeResponse.DynamicConfigs {
		for j := range clientInitializeResponse.DynamicConfigs[i].SecondaryExposures {
			clientInitializeResponse.DynamicConfigs[i].SecondaryExposures[j].Gate = "__REMOVED_FOR_TEST__"
		}
	}
	for i := range clientInitializeResponse.LayerConfigs {
		for j := range clientInitializeResponse.LayerConfigs[i].SecondaryExposures {
			clientInitializeResponse.LayerConfigs[i].SecondaryExposures[j].Gate = "__REMOVED_FOR_TEST__"
		}
		for j := range clientInitializeResponse.LayerConfigs[i].UndelegatedSecondaryExposures {
			clientInitializeResponse.LayerConfigs[i].UndelegatedSecondaryExposures[j].Gate = "__REMOVED_FOR_TEST__"
		}
	}
	clientInitializeResponse.Generator = "__REMOVED_FOR_TEST__"
	clientInitializeResponse.Time = 0
	clientInitializeResponse.SDKInfo = SDKInfo{}
	clientInitializeResponse.User = User{}
}
