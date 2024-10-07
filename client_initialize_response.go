package statsig

import (
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
	SDKInfo        SDKInfo                             `json:"sdkInfo"`
	User           User                                `json:"user"`
	HashUsed       string                              `json:"hash_used"`
}

type SDKInfo struct {
	SDKType    string `json:"sdkType"`
	SDKVersion string `json:"sdkVersion"`
}

type baseSpecInitializeResponse struct {
	Name               string              `json:"name"`
	RuleID             string              `json:"rule_id"`
	SecondaryExposures []SecondaryExposure `json:"secondary_exposures"`
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
	GroupName          string                 `json:"group_name,omitempty"`
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
	UndelegatedSecondaryExposures []SecondaryExposure    `json:"undelegated_secondary_exposures"`
	GroupName                     string                 `json:"group_name,omitempty"`
}

func mergeMaps(a map[string]interface{}, b map[string]interface{}) {
	for k, v := range b {
		a[k] = v
	}
}

func getClientInitializeResponse(
	user User,
	e *evaluator,
	context *evalContext,
) ClientInitializeResponse {
	hashAlgorithm := context.Hash
	if hashAlgorithm != "none" && hashAlgorithm != "djb2" {
		hashAlgorithm = "sha256"
	}

	evalResultToBaseResponse := func(name string, eval *evalResult) (string, baseSpecInitializeResponse) {
		hashedName := hashName(hashAlgorithm, name)
		result := baseSpecInitializeResponse{
			Name:               hashedName,
			RuleID:             eval.RuleID,
			SecondaryExposures: eval.SecondaryExposures,
		}
		return hashedName, result
	}
	gateToResponse := func(gateName string, spec configSpec) (string, GateInitializeResponse) {
		evalRes := &evalResult{}
		if context.IncludeLocalOverrides {
			if gateOverride, hasOverride := e.getGateOverrideEval(gateName); hasOverride {
				evalRes = gateOverride
			} else {
				evalRes = e.eval(user, spec, 0, context)
			}
		} else {
			evalRes = e.eval(user, spec, 0, context)
		}
		hashedName, base := evalResultToBaseResponse(gateName, evalRes)
		result := GateInitializeResponse{
			baseSpecInitializeResponse: base,
			Value:                      evalRes.Value,
		}
		return hashedName, result
	}
	configToResponse := func(configName string, spec configSpec) (string, ConfigInitializeResponse) {
		evalRes := &evalResult{}
		if context.IncludeLocalOverrides {
			if configOverride, hasOverride := e.getConfigOverrideEval(configName); hasOverride {
				evalRes = configOverride
			} else {
				evalRes = e.eval(user, spec, 0, context)
			}
		} else {
			evalRes = e.eval(user, spec, 0, context)
		}
		hashedName, base := evalResultToBaseResponse(configName, evalRes)
		result := ConfigInitializeResponse{
			baseSpecInitializeResponse: base,
			Value:                      evalRes.JsonValue,
			Group:                      evalRes.RuleID,
			IsDeviceBased:              strings.EqualFold(spec.IDType, "stableid"),
		}
		if evalRes.GroupName != "" {
			result.GroupName = evalRes.GroupName
		}
		if strings.EqualFold(spec.Entity, "experiment") {
			result.IsUserInExperiment = new(bool)
			*result.IsUserInExperiment = evalRes.IsExperimentGroup != nil && *evalRes.IsExperimentGroup
			result.IsExperimentActive = new(bool)
			*result.IsExperimentActive = spec.IsActive != nil && *spec.IsActive
			if spec.HasSharedParams != nil && *spec.HasSharedParams {
				result.IsInLayer = new(bool)
				*result.IsInLayer = true
				result.ExplicitParameters = new([]string)
				*result.ExplicitParameters = spec.ExplicitParameters
				layerName, _ := e.store.getExperimentLayer(spec.Name)
				layer, exists := e.store.getLayerConfig(layerName)
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
		evalResult := e.eval(user, spec, 0, &evalContext{Hash: hashAlgorithm})
		hashedName, base := evalResultToBaseResponse(layerName, evalResult)
		result := LayerInitializeResponse{
			baseSpecInitializeResponse:    base,
			Value:                         evalResult.JsonValue,
			Group:                         evalResult.RuleID,
			IsDeviceBased:                 strings.EqualFold(spec.IDType, "stableid"),
			UndelegatedSecondaryExposures: evalResult.UndelegatedSecondaryExposures,
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
			delegateSpec, exists := e.store.getDynamicConfig(delegate)
			delegateResult := e.eval(user, delegateSpec, 0, context)
			if exists {
				result.AllocatedExperimentName = hashName(hashAlgorithm, delegate)
				result.IsUserInExperiment = new(bool)
				*result.IsUserInExperiment = delegateResult.IsExperimentGroup != nil && *delegateResult.IsExperimentGroup
				result.IsExperimentActive = new(bool)
				*result.IsExperimentActive = delegateSpec.IsActive != nil && *delegateSpec.IsActive
				if len(delegateSpec.ExplicitParameters) > 0 {
					*result.ExplicitParameters = delegateSpec.ExplicitParameters
				}
				if delegateResult.GroupName != "" {
					result.GroupName = delegateResult.GroupName
				}
			}
		}
		return hashedName, result
	}

	var appId string
	if context.TargetAppID != "" {
		appId = context.TargetAppID
	} else {
		appId, _ = e.store.getAppIDForSDKKey(context.ClientKey)
	}

	filterByEntities := false
	gatesLookup := make(map[string]bool)
	configsLookup := make(map[string]bool)
	if entities, ok := e.store.getEntitiesForSDKKey(context.ClientKey); ok {
		filterByEntities = true
		for _, gate := range entities.Gates {
			gatesLookup[gate] = true
		}
		for _, config := range entities.Configs {
			configsLookup[config] = true
		}
	}

	featureGates := make(map[string]GateInitializeResponse)
	dynamicConfigs := make(map[string]ConfigInitializeResponse)
	layerConfigs := make(map[string]LayerInitializeResponse)
	for name, spec := range e.store.featureGates {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		if filterByEntities {
			if _, ok := gatesLookup[name]; !ok {
				continue
			}
		}
		if !strings.EqualFold(spec.Entity, "segment") && !strings.EqualFold(spec.Entity, "holdout") {
			hashedName, res := gateToResponse(name, spec)
			featureGates[hashedName] = res
		}
	}
	for name, spec := range e.store.dynamicConfigs {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		if filterByEntities {
			if _, ok := configsLookup[name]; !ok {
				continue
			}
		}
		hashedName, res := configToResponse(name, spec)
		dynamicConfigs[hashedName] = res
	}
	for name, spec := range e.store.layerConfigs {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		hashedName, res := layerToResponse(name, spec)
		layerConfigs[hashedName] = res
	}

	meta := getStatsigMetadata()

	response := ClientInitializeResponse{
		FeatureGates:   featureGates,
		DynamicConfigs: dynamicConfigs,
		LayerConfigs:   layerConfigs,
		SdkParams:      make(map[string]string),
		HasUpdates:     true,
		Generator:      "statsig-go-sdk",
		EvaluatedKeys:  map[string]interface{}{"userID": user.UserID, "customIDs": user.CustomIDs},
		Time:           e.store.lastSyncTime,
		SDKInfo:        SDKInfo{SDKVersion: meta.SDKVersion, SDKType: meta.SDKType},
		User:           *user.getCopyForLogging(),
		HashUsed:       hashAlgorithm,
	}
	return response
}
