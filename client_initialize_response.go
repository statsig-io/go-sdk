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

type BaseSpecInitializeResponse struct {
	Name               string              `json:"name"`
	RuleID             string              `json:"rule_id"`
	IDType             string              `json:"id_type,omitempty"`
	SecondaryExposures []SecondaryExposure `json:"secondary_exposures"`
	ConfigType         ConfigType          `json:"config_type,omitempty"`
}

type GateInitializeResponse struct {
	BaseSpecInitializeResponse
	Value bool `json:"value"`
}

type ConfigInitializeResponse struct {
	BaseSpecInitializeResponse
	Value              map[string]interface{} `json:"value"`
	Group              string                 `json:"group"`
	IsDeviceBased      bool                   `json:"is_device_based"`
	IsExperimentActive *bool                  `json:"is_experiment_active,omitempty"`
	IsUserInExperiment *bool                  `json:"is_user_in_experiment,omitempty"`
	IsInLayer          *bool                  `json:"is_in_layer,omitempty"`
	ExplicitParameters *[]string              `json:"explicit_parameters,omitempty"`
	GroupName          string                 `json:"group_name,omitempty"`
	RulePassed         bool                   `json:"passed"`
	IsControlGroup     *bool                  `json:"is_control_group,omitempty"`
}

type LayerInitializeResponse struct {
	BaseSpecInitializeResponse
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

func getConfigType(spec configSpec) ConfigType {
	switch spec.Entity {
	case "feature_gate":
		return FeatureGateType
	case "holdout":
		return HoldoutType
	case "segment":
		return SegmentType
	case "dynamic_config":
		return DynamicConfigType
	case "experiment":
		return ExperimentType
	case "autotune":
		return AutotuneType
	case "layer":
		return LayerType
	default:
		return UnknownType
	}
}

func getClientInitializeResponse(
	user User,
	e *evaluator,
	context *evalContext,
	options *GCIROptions,
) ClientInitializeResponse {
	hashAlgorithm := context.Hash
	if hashAlgorithm != "none" && hashAlgorithm != "djb2" {
		hashAlgorithm = "sha256"
	}

	var appId string
	if context.TargetAppID != "" {
		appId = context.TargetAppID
	} else {
		appId, _ = e.store.getAppIDForSDKKey(context.ClientKey)
		context.TargetAppID = appId
	}

	evalResultToBaseResponse := func(name string, eval *evalResult) (string, BaseSpecInitializeResponse) {
		hashedName := hashName(hashAlgorithm, name)
		result := BaseSpecInitializeResponse{
			Name:               hashedName,
			RuleID:             eval.RuleID,
			IDType:             eval.IDType,
			SecondaryExposures: eval.SecondaryExposures,
		}
		return hashedName, result
	}
	getExperimentControlRule := func(name string, spec configSpec) *configRule {
		for _, rule := range spec.Rules {
			if rule.IsControlGroup != nil && *rule.IsControlGroup {
				return &rule
			}
		}
		return nil
	}
	gateToResponse := func(gateName string, spec configSpec) (string, GateInitializeResponse) {
		evalRes := &evalResult{}
		if context.IncludeLocalOverrides || options.Overrides != nil {
			if gateOverride, hasOverride := e.getGateOverrideEvalForGCIR(gateName, options); hasOverride {
				evalRes = gateOverride
			} else {
				evalRes = e.eval(user, spec, 0, context)
			}
		} else {
			evalRes = e.eval(user, spec, 0, context)
		}
		hashedName, base := evalResultToBaseResponse(gateName, evalRes)
		result := GateInitializeResponse{
			BaseSpecInitializeResponse: base,
			Value:                      evalRes.Value,
		}
		if context.IncludeConfigType {
			result.ConfigType = getConfigType(spec)
		}
		return hashedName, result
	}

	configToResponse := func(configName string, spec configSpec) (string, ConfigInitializeResponse) {
		evalRes := &evalResult{}
		hasExpOverride := false
		if context.IncludeLocalOverrides || options.Overrides != nil {
			if configOverride, hasOverride := e.getConfigOverrideEvalForGCIR(spec, options); hasOverride {
				hasExpOverride = true
				evalRes = configOverride
			} else {
				evalRes = e.eval(user, spec, 0, context)
			}
		} else {
			evalRes = e.eval(user, spec, 0, context)
		}
		hashedName, base := evalResultToBaseResponse(configName, evalRes)
		result := ConfigInitializeResponse{
			BaseSpecInitializeResponse: base,
			Value:                      evalRes.JsonValue,
			Group:                      evalRes.RuleID,
			IsDeviceBased:              strings.EqualFold(spec.IDType, "stableid"),
			RulePassed:                 evalRes.Value,
		}
		if context.IncludeConfigType {
			result.ConfigType = getConfigType(spec)
		}
		if evalRes.GroupName != "" {
			result.GroupName = evalRes.GroupName
		}
		if strings.EqualFold(spec.Entity, "experiment") {
			result.IsUserInExperiment = new(bool)
			*result.IsUserInExperiment = evalRes.IsExperimentGroup != nil && *evalRes.IsExperimentGroup
			result.IsExperimentActive = new(bool)
			*result.IsExperimentActive = spec.IsActive != nil && *spec.IsActive
			if context.UseControlForUsersNotInExperiment {
				controlRule := getExperimentControlRule(configName, spec)
				if controlRule != nil && !*result.IsUserInExperiment && !hasExpOverride {
					result.Value = controlRule.ReturnValueJSON
					result.GroupName = controlRule.GroupName
				}
				if controlRule != nil && strings.EqualFold(controlRule.GroupName, result.GroupName) {
					result.IsControlGroup = new(bool)
					*result.IsControlGroup = true
				}
			}
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
		evalResult := &evalResult{}
		if context.IncludeLocalOverrides || options.Overrides != nil {
			if layerOverride, hasOverride := e.getLayerOverrideEvalForGCIR(spec, options); hasOverride {
				evalResult = layerOverride
			} else {
				evalResult = e.eval(user, spec, 0, context)
			}
		} else {
			evalResult = e.eval(user, spec, 0, context)
		}
		hashedName, base := evalResultToBaseResponse(layerName, evalResult)
		result := LayerInitializeResponse{
			BaseSpecInitializeResponse:    base,
			Value:                         evalResult.JsonValue,
			Group:                         evalResult.RuleID,
			IsDeviceBased:                 strings.EqualFold(spec.IDType, "stableid"),
			UndelegatedSecondaryExposures: evalResult.UndelegatedSecondaryExposures,
		}
		if context.IncludeConfigType {
			result.ConfigType = getConfigType(spec)
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

	configTypesMap := make(map[ConfigType]struct{})
	if context.ConfigTypesToInclude != nil {
		for _, configType := range context.ConfigTypesToInclude {
			configTypesMap[configType] = struct{}{}
		}
	}

	shouldFilterConfigType := func(specType ConfigType) bool {
		if context.ConfigTypesToInclude == nil {
			return false
		}
		if _, exists := configTypesMap[specType]; exists {
			return false
		}
		return true

	}

	featureGates := make(map[string]GateInitializeResponse)
	dynamicConfigs := make(map[string]ConfigInitializeResponse)
	layerConfigs := make(map[string]LayerInitializeResponse)
	for name, spec := range e.store.featureGates {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		if shouldFilterConfigType(getConfigType(spec)) {
			continue
		}
		if filterByEntities {
			if _, ok := gatesLookup[name]; !ok {
				continue
			}
		}
		if !strings.EqualFold(spec.Entity, SegmentType) && !strings.EqualFold(spec.Entity, HoldoutType) {
			hashedName, res := gateToResponse(name, spec)
			featureGates[hashedName] = res
		}
	}
	for name, spec := range e.store.dynamicConfigs {
		if !spec.hasTargetAppID(appId) {
			continue
		}
		if shouldFilterConfigType(getConfigType(spec)) {
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
		if shouldFilterConfigType(getConfigType(spec)) {
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
