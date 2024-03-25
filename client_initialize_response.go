package statsig

import (
	"fmt"
	"strings"
)

type ClientInitializeResponse struct {
	FeatureGates   map[string]GateInitializeResponse   `json:"feature_gates"`
	DynamicConfigs map[string]ConfigInitializeResponse `json:"dynamic_configs"`
	LayerConfigs   map[string]LayerInitializeResponse  `json:"layer_configs"`
	SdkParams      map[string]string                   `json:"sdkParams"`
	HasUpdates     bool                                `json:"has_updates"`
	Generator      string                              `json:"generator"`
	EvaluatedKeys  map[string]interface{}              `json:"evaluated_keys"`
	Time           int64                               `json:"time"`
}

type baseSpecInitializeResponse struct {
	Name               string              `json:"name"`
	RuleID             string              `json:"rule_id"`
	SecondaryExposures []map[string]string `json:"secondary_exposures"`
}

type GateInitializeResponse struct {
	baseSpecInitializeResponse
	Value bool `json:"value"`
}

type ConfigInitializeResponse struct {
	baseSpecInitializeResponse
	Value              map[string]interface{} `json:"value"`
	Group              string                 `json:"group"`
	IsDeviceBased      bool                   `json:"is_device_based"`
	IsExperimentActive *bool                  `json:"is_experiment_active,omitempty"`
	IsUserInExperiment *bool                  `json:"is_user_in_experiment,omitempty"`
	IsInLayer          *bool                  `json:"is_in_layer,omitempty"`
	ExplicitParameters *[]string              `json:"explicit_parameters,omitempty"`
}

type LayerInitializeResponse struct {
	baseSpecInitializeResponse
	Value                         map[string]interface{} `json:"value"`
	Group                         string                 `json:"group"`
	IsDeviceBased                 bool                   `json:"is_device_based"`
	IsExperimentActive            *bool                  `json:"is_experiment_active,omitempty"`
	IsUserInExperiment            *bool                  `json:"is_user_in_experiment,omitempty"`
	ExplicitParameters            *[]string              `json:"explicit_parameters,omitempty"`
	AllocatedExperimentName       string                 `json:"allocated_experiment_name,omitempty"`
	UndelegatedSecondaryExposures []map[string]string    `json:"undelegated_secondary_exposures"`
}

func cleanExposures(exposures []map[string]string) []map[string]string {
	seen := make(map[string]bool)
	result := make([]map[string]string, 0)
	for _, exposure := range exposures {
		key := fmt.Sprintf("%s|%s|%s", exposure["gate"], exposure["gateValue"], exposure["ruleID"])
		if _, exists := seen[key]; !exists {
			seen[key] = true
			result = append(result, exposure)
		}
	}
	return result
}

func mergeMaps(a map[string]interface{}, b map[string]interface{}) {
	for k, v := range b {
		a[k] = v
	}
}

func getClientInitializeResponse(
	user User,
	store *store,
	evalFunc func(user User, spec configSpec, depth int) *evalResult,
	clientKey string,
) ClientInitializeResponse {
	evalResultToBaseResponse := func(name string, eval *evalResult) (string, baseSpecInitializeResponse) {
		hashedName := getHashBase64StringEncoding(name)
		result := baseSpecInitializeResponse{
			Name:               hashedName,
			RuleID:             eval.RuleID,
			SecondaryExposures: cleanExposures(eval.SecondaryExposures),
		}
		return hashedName, result
	}
	gateToResponse := func(gateName string, spec configSpec) (string, GateInitializeResponse) {
		evalResult := evalFunc(user, spec, 0)
		hashedName, base := evalResultToBaseResponse(gateName, evalResult)
		result := GateInitializeResponse{
			baseSpecInitializeResponse: base,
			Value:                      evalResult.Pass,
		}
		return hashedName, result
	}
	configToResponse := func(configName string, spec configSpec) (string, ConfigInitializeResponse) {
		evalResult := evalFunc(user, spec, 0)
		hashedName, base := evalResultToBaseResponse(configName, evalResult)
		result := ConfigInitializeResponse{
			baseSpecInitializeResponse: base,
			Value:                      evalResult.ConfigValue.Value,
			Group:                      evalResult.RuleID,
			IsDeviceBased:              strings.ToLower(spec.IDType) == "stableid",
		}
		entityType := strings.ToLower(spec.Entity)
		if entityType == "experiment" {
			result.IsUserInExperiment = new(bool)
			*result.IsUserInExperiment = evalResult.IsExperimentGroup != nil && *evalResult.IsExperimentGroup
			result.IsExperimentActive = new(bool)
			*result.IsExperimentActive = spec.IsActive != nil && *spec.IsActive
			if spec.HasSharedParams != nil && *spec.HasSharedParams {
				result.IsInLayer = new(bool)
				*result.IsInLayer = true
				result.ExplicitParameters = new([]string)
				*result.ExplicitParameters = spec.ExplicitParameters
				layerName, _ := store.getExperimentLayer(spec.Name)
				layer, exists := store.getLayerConfig(layerName)
				defaultValue := make(map[string]interface{})
				if exists {
					mergeMaps(defaultValue, layer.DefaultValueJSON)
					mergeMaps(defaultValue, result.Value)
					result.Value = defaultValue
				}
			}
		}
		return hashedName, result
	}
	layerToResponse := func(layerName string, spec configSpec) (string, LayerInitializeResponse) {
		evalResult := evalFunc(user, spec, 0)
		hashedName, base := evalResultToBaseResponse(layerName, evalResult)
		result := LayerInitializeResponse{
			baseSpecInitializeResponse:    base,
			Value:                         evalResult.ConfigValue.Value,
			Group:                         evalResult.RuleID,
			IsDeviceBased:                 strings.ToLower(spec.IDType) == "stableid",
			UndelegatedSecondaryExposures: cleanExposures(evalResult.UndelegatedSecondaryExposures),
		}
		delegate := evalResult.ConfigDelegate
		result.ExplicitParameters = new([]string)
		if len(spec.ExplicitParameters) > 0 {
			// spec.ExplicitParameters may be "null" due to how
			// JSON Unmarshal works with fields with unallocated memory
			*result.ExplicitParameters = spec.ExplicitParameters
		} else {
			*result.ExplicitParameters = make([]string, 0)
		}
		if delegate != "" {
			delegateSpec, exists := store.getDynamicConfig(delegate)
			delegateResult := evalFunc(user, delegateSpec, 0)
			if exists {
				result.AllocatedExperimentName = getHashBase64StringEncoding(delegate)
				result.IsUserInExperiment = new(bool)
				*result.IsUserInExperiment = delegateResult.IsExperimentGroup != nil && *delegateResult.IsExperimentGroup
				result.IsExperimentActive = new(bool)
				*result.IsExperimentActive = delegateSpec.IsActive != nil && *delegateSpec.IsActive
				if len(delegateSpec.ExplicitParameters) > 0 {
					*result.ExplicitParameters = delegateSpec.ExplicitParameters
				}
			}
		}
		return hashedName, result
	}

	appId, _ := store.getAppIDForSDKKey(clientKey)
	featureGates := make(map[string]GateInitializeResponse)
	dynamicConfigs := make(map[string]ConfigInitializeResponse)
	layerConfigs := make(map[string]LayerInitializeResponse)
	for name, spec := range store.featureGates {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		entityType := strings.ToLower(spec.Entity)
		if entityType != "segment" && entityType != "holdout" {
			hashedName, res := gateToResponse(name, spec)
			featureGates[hashedName] = res
		}
	}
	for name, spec := range store.dynamicConfigs {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		hashedName, res := configToResponse(name, spec)
		dynamicConfigs[hashedName] = res
	}
	for name, spec := range store.layerConfigs {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		hashedName, res := layerToResponse(name, spec)
		layerConfigs[hashedName] = res
	}

	response := ClientInitializeResponse{
		FeatureGates:   featureGates,
		DynamicConfigs: dynamicConfigs,
		LayerConfigs:   layerConfigs,
		SdkParams:      make(map[string]string),
		HasUpdates:     true,
		Generator:      "statsig-go-sdk",
		EvaluatedKeys:  map[string]interface{}{"userID": user.UserID, "customIDs": user.CustomIDs},
		Time:           0,
	}
	return response
}
