package statsig

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type evalResult struct {
	Value                         bool                   `json:"value"`
	JsonValue                     map[string]interface{} `json:"json_value"`
	Unsupported                   bool                   `json:"unsupported"`
	RuleID                        string                 `json:"rule_id"`
	IDType                        string                 `json:"id_type"`
	GroupName                     string                 `json:"group_name"`
	SecondaryExposures            []SecondaryExposure    `json:"secondary_exposures"`
	UndelegatedSecondaryExposures []SecondaryExposure    `json:"undelegated_secondary_exposures"`
	ConfigDelegate                string                 `json:"config_delegate"`
	ExplicitParameters            []string               `json:"explicit_parameters"`
	EvaluationDetails             *EvaluationDetails     `json:"evaluation_details,omitempty"`
	IsExperimentGroup             *bool                  `json:"is_experiment_group,omitempty"`
	DerivedDeviceMetadata         *DerivedDeviceMetadata `json:"derived_device_metadata,omitempty"`
	ForwardAllExposures           bool                   `json:"forward_all_exposures,omitempty"`
	SamplingRate                  *int                   `json:"sampling_rate,omitempty"`
	ConfigVersion                 *int                   `json:"config_version,omitempty"`
	HasSeenAnalyticalGates        bool                   `json:"has_seen_analytical_gates,omitempty"`
	TargetAppIDs                  []string               `json:"target_app_ids,omitempty"`
}

type DerivedDeviceMetadata struct {
	OsName         string `json:"os_name"`
	OsVersion      string `json:"os_version"`
	BrowserName    string `json:"browser_name"`
	BrowserVersion string `json:"browser_version"`
}

type SecondaryExposure struct {
	Gate      string `json:"gate"`
	GateValue string `json:"gateValue"`
	RuleID    string `json:"ruleID"`
}

func newEvalResultFromUserPersistedValues(configName string, persitedValues UserPersistedValues) *evalResult {
	if stickyValues, ok := persitedValues[configName]; ok {
		newEvalResult := newEvalResultFromStickyValues(stickyValues)
		return newEvalResult
	}
	return nil
}

func newEvalResultFromStickyValues(evalMap StickyValues) *evalResult {
	evaluationDetails := reconstructEvaluationDetailsFromPersisted(
		safeParseJSONint64(evalMap.Time),
	)

	return &evalResult{
		Value:                         evalMap.Value,
		RuleID:                        evalMap.RuleID,
		GroupName:                     evalMap.GroupName,
		SecondaryExposures:            evalMap.SecondaryExposures,
		JsonValue:                     evalMap.JsonValue,
		EvaluationDetails:             evaluationDetails,
		ConfigDelegate:                evalMap.ConfigDelegate,
		ExplicitParameters:            evalMap.ExplicitParameters,
		UndelegatedSecondaryExposures: evalMap.UndelegatedSecondaryExposures,
	}
}

func (e *evalResult) toStickyValues() StickyValues {
	return StickyValues{
		Value:                         e.Value,
		JsonValue:                     e.JsonValue,
		RuleID:                        e.RuleID,
		GroupName:                     e.GroupName,
		SecondaryExposures:            e.SecondaryExposures,
		Time:                          e.EvaluationDetails.ConfigSyncTime,
		ConfigDelegate:                e.ConfigDelegate,
		ExplicitParameters:            e.ExplicitParameters,
		UndelegatedSecondaryExposures: e.UndelegatedSecondaryExposures,
	}
}

type evaluator struct {
	store                  *store
	gateOverrides          map[string]bool
	configOverrides        map[string]map[string]interface{}
	layerOverrides         map[string]map[string]interface{}
	countryLookup          *countryLookup
	uaParser               *uaParser
	persistentStorageUtils *userPersistentStorageUtils
	mu                     sync.RWMutex
}

const dynamicConfigType = "dynamic_config"
const maxRecursiveDepth = 300

func newEvaluator(
	transport *transport,
	errorBoundary *errorBoundary,
	options *Options,
	diagnostics *diagnostics,
	sdkKey string,
	sdkConfigs *SDKConfigs,
	context *initContext,
) *evaluator {
	store := newStore(transport, errorBoundary, options, diagnostics, sdkKey, sdkConfigs, context)
	defer func() {
		if err := recover(); err != nil {
			errorBoundary.logException(toError(err))
			Logger().LogError(err)
		}
	}()
	persistentStorageUtils := newUserPersistentStorageUtils(options)
	countryLookup := newCountryLookup(options.IPCountryOptions)
	uaParser := newUAParser(options.UAParserOptions)

	return &evaluator{
		store:                  store,
		countryLookup:          countryLookup,
		uaParser:               uaParser,
		gateOverrides:          make(map[string]bool),
		configOverrides:        make(map[string]map[string]interface{}),
		layerOverrides:         make(map[string]map[string]interface{}),
		persistentStorageUtils: persistentStorageUtils,
	}
}

func (e *evaluator) initialize(context *initContext) {
	e.store.initialize(context)
	e.uaParser.init()
	e.countryLookup.init()
}

func (e *evaluator) shutdown() {
	if e.store.dataAdapter != nil {
		e.store.dataAdapter.Shutdown()
	}
	e.store.stopPolling()
}

func (e *evaluator) createEvaluationDetails(reason EvaluationReason) *EvaluationDetails {
	e.store.mu.RLock()
	defer e.store.mu.RUnlock()
	return newEvaluationDetails(e.store.source, reason, e.store.lastSyncTime, e.store.initialSyncTime)
}

func (e *evaluator) getDynamicConfig(configName string) (configSpec, bool) {
	return e.store.getDynamicConfig(configName)
}

func (e *evaluator) evalGate(user User, gateName string, context *evalContext) *evalResult {
	return e.evalGateImpl(user, gateName, 0, context)
}

func (e *evaluator) evalGateImpl(user User, gateName string, depth int, context *evalContext) *evalResult {
	if gateOverrideEval, hasOverride := e.getGateOverrideEval(gateName); hasOverride {
		return gateOverrideEval
	}
	if gate, hasGate := e.store.getGate(gateName); hasGate {
		return e.eval(user, gate, depth, context)
	}
	emptyEvalResult := new(evalResult)
	emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(ReasonUnrecognized)
	emptyEvalResult.SecondaryExposures = make([]SecondaryExposure, 0)
	return emptyEvalResult
}

func (e *evaluator) evalConfig(user User, configName string, context *evalContext) *evalResult {
	return e.evalConfigImpl(user, configName, 0, context)
}

func (e *evaluator) evalConfigImpl(user User, configName string, depth int, context *evalContext) *evalResult {
	if configOverrideEval, hasOverride := e.getConfigOverrideEval(configName); hasOverride {
		return configOverrideEval
	}

	config, hasConfig := e.store.getDynamicConfig(configName)
	if !hasConfig {
		_, hasCMAB := e.store.getCMABConfig(configName)
		if hasCMAB {
			return e.evalCMABImpl(user, configName, depth, context)
		}

		emptyEvalResult := new(evalResult)
		emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(ReasonUnrecognized)
		emptyEvalResult.SecondaryExposures = make([]SecondaryExposure, 0)
		return emptyEvalResult
	}

	if context.PersistedValues == nil || config.IsActive == nil || !*config.IsActive {
		return e.evalAndDeleteFromPersistentStorage(user, config, depth, context)
	}

	stickyResult := newEvalResultFromUserPersistedValues(configName, context.PersistedValues)
	if stickyResult != nil {
		return stickyResult
	}

	return e.evalAndSaveToPersistentStorage(user, config, depth, context)
}

func (e *evaluator) evalLayer(user User, name string, context *evalContext) *evalResult {
	return e.evalLayerImpl(user, name, 0, context)
}

func (e *evaluator) evalLayerImpl(user User, name string, depth int, context *evalContext) *evalResult {
	if layerOverrideEval, hasOverride := e.getLayerOverrideEval(name); hasOverride {
		return layerOverrideEval
	}
	config, hasConfig := e.store.getLayerConfig(name)
	if !hasConfig {
		emptyEvalResult := new(evalResult)
		emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(ReasonUnrecognized)
		emptyEvalResult.SecondaryExposures = make([]SecondaryExposure, 0)
		return emptyEvalResult
	}

	if context.PersistedValues == nil {
		return e.evalAndDeleteFromPersistentStorage(user, config, depth, context)
	}

	stickyResult := newEvalResultFromUserPersistedValues(name, context.PersistedValues)
	if stickyResult != nil {
		if e.allocatedExperimentExistsAndIsActive(stickyResult) {
			return stickyResult
		} else {
			return e.evalAndDeleteFromPersistentStorage(user, config, depth, context)
		}
	} else {
		evaluation := e.eval(user, config, depth, context)
		if e.allocatedExperimentExistsAndIsActive(evaluation) {
			if evaluation.IsExperimentGroup != nil && *evaluation.IsExperimentGroup {
				e.persistentStorageUtils.save(user, config.IDType, name, evaluation)
			}
		} else {
			e.persistentStorageUtils.delete(user, config.IDType, name)
		}
		return evaluation
	}
}

func (e *evaluator) evalCMABImpl(user User, cmabName string, depth int, context *evalContext) *evalResult {
	cmab, hasCMAB := e.store.getCMABConfig(cmabName)
	if !hasCMAB {
		emptyEvalResult := new(evalResult)
		emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(ReasonUnrecognized)
		emptyEvalResult.SecondaryExposures = make([]SecondaryExposure, 0)
		return emptyEvalResult
	}

	if !cmab.Enabled || len(cmab.Groups) == 0 {
		result := &evalResult{
			JsonValue:         cmab.DefaultValueJSON,
			RuleID:            "prestart",
			EvaluationDetails: e.createEvaluationDetails(ReasonNone),
		}
		e.finalizeCMABEvalResult(&cmab, result, false)
		return result
	}

	targetingGateName := cmab.TargetingGateName

	var exposures []SecondaryExposure
	if targetingGateName != nil && *targetingGateName != "" {
		gateResult := e.evalGateImpl(user, *targetingGateName, depth+1, context)
		if !context.DisableLogExposures {
			newExposure := SecondaryExposure{
				Gate:      *targetingGateName,
				GateValue: strconv.FormatBool(gateResult.Value),
				RuleID:    gateResult.RuleID,
			}
			exposures = append(exposures, newExposure)
		}

		if !gateResult.Value {
			result := &evalResult{
				JsonValue:          cmab.DefaultValueJSON,
				RuleID:             "inlineTargetingRules",
				SecondaryExposures: exposures,
				EvaluationDetails:  e.createEvaluationDetails(ReasonNone),
			}
			e.finalizeCMABEvalResult(&cmab, result, false)
			return result
		}
	}

	unitID := getUnitID(user, cmab.IDType)
	salt := cmab.Salt
	if salt == "" {
		salt = cmabName
	}
	hash := getHashUint64Encoding(salt + "." + unitID)

	cmabConfig := cmab.Config
	if cmabConfig == nil {
		denom := math.Max(1.0, float64(len(cmab.Groups)))
		groupSize := 10000.0 / denom
		groupIndex := int(float64(hash%10000) / groupSize)
		group := cmab.Groups[groupIndex]
		isExperimentGroup := true
		result := &evalResult{
			JsonValue:          group.ParameterValuesJSON,
			RuleID:             group.ID + ":explore",
			GroupName:          group.Name,
			IsExperimentGroup:  &isExperimentGroup,
			SecondaryExposures: exposures,
			EvaluationDetails:  e.createEvaluationDetails(ReasonNone),
		}
		e.finalizeCMABEvalResult(&cmab, result, true)
		return result
	}

	shouldSample := float64(hash%10000) < cmab.SampleRate*10000
	if shouldSample {
		if samplingResult := e.applyCMABSampling(&cmab, cmabConfig, exposures); samplingResult != nil {
			e.finalizeCMABEvalResult(&cmab, samplingResult, true)
			return samplingResult
		}
	}

	bestGroupResult := e.applyCMABBestGroup(&cmab, cmabConfig, user, exposures)
	e.finalizeCMABEvalResult(&cmab, bestGroupResult, true)
	return bestGroupResult
}

func (e *evaluator) allocatedExperimentExistsAndIsActive(evaluation *evalResult) bool {
	delegate, exists := e.store.getDynamicConfig(evaluation.ConfigDelegate)
	return exists && delegate.IsActive != nil && *delegate.IsActive
}

func (e *evaluator) evalAndSaveToPersistentStorage(user User, config configSpec, depth int, context *evalContext) *evalResult {
	evaluation := e.eval(user, config, depth, context)
	if evaluation.IsExperimentGroup != nil && *evaluation.IsExperimentGroup {
		e.persistentStorageUtils.save(user, config.IDType, config.Name, evaluation)
	}
	return evaluation
}

func (e *evaluator) evalAndDeleteFromPersistentStorage(user User, config configSpec, depth int, context *evalContext) *evalResult {
	e.persistentStorageUtils.delete(user, config.IDType, config.Name)
	return e.eval(user, config, depth, context)
}

func (e *evaluator) getGateOverride(name string) (bool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	gate, ok := e.gateOverrides[name]
	return gate, ok
}

func (e *evaluator) getGateOverrideEval(name string) (*evalResult, bool) {
	if gateOverride, hasOverride := e.getGateOverride(name); hasOverride {
		evalDetails := e.createEvaluationDetails(ReasonLocalOverride)
		return &evalResult{
			Value:              gateOverride,
			RuleID:             "override",
			EvaluationDetails:  evalDetails,
			SecondaryExposures: make([]SecondaryExposure, 0),
		}, true
	}

	return &evalResult{}, false
}

func (e *evaluator) getConfigOverride(name string) (map[string]interface{}, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	config, ok := e.configOverrides[name]
	return config, ok
}

func (e *evaluator) getConfigOverrideEval(name string) (*evalResult, bool) {
	if configOverride, hasOverride := e.getConfigOverride(name); hasOverride {
		evalDetails := e.createEvaluationDetails(ReasonLocalOverride)
		return &evalResult{
			Value:              true,
			JsonValue:          configOverride,
			RuleID:             "override",
			EvaluationDetails:  evalDetails,
			SecondaryExposures: make([]SecondaryExposure, 0),
		}, true
	}

	return &evalResult{}, false
}

func (e *evaluator) getLayerOverride(name string) (map[string]interface{}, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	layer, ok := e.layerOverrides[name]
	return layer, ok
}

func (e *evaluator) getLayerOverrideEval(name string) (*evalResult, bool) {
	if layerOverride, hasOverride := e.getLayerOverride(name); hasOverride {
		evalDetails := e.createEvaluationDetails(ReasonLocalOverride)
		return &evalResult{
			Value:              true,
			JsonValue:          layerOverride,
			RuleID:             "override",
			EvaluationDetails:  evalDetails,
			SecondaryExposures: make([]SecondaryExposure, 0),
		}, true
	}

	return &evalResult{}, false
}

// Override the value of a Feature Gate for the given user
func (e *evaluator) OverrideGate(gate string, val bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gateOverrides[gate] = val
}

// Override the DynamicConfig value for the given user
func (e *evaluator) OverrideConfig(config string, val map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.configOverrides[config] = val
}

// Override the Layer value for the given user
func (e *evaluator) OverrideLayer(layer string, val map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.layerOverrides[layer] = val
}

func (e *evaluator) getGateOverrideEvalForGCIR(gateName string, options *GCIROptions) (*evalResult, bool) {
	if options.Overrides != nil && options.Overrides.FeatureGate != nil {
		gateOptionOverride := options.Overrides.FeatureGate
		if gateOptionOverride != nil {
			if value, exists := gateOptionOverride[gateName]; exists {
				return &evalResult{
					Value:  value,
					RuleID: "override",
				}, true
			}
		}
	}

	if options.IncludeLocalOverrides {

		return e.getGateOverrideEval(gateName)
	}

	return &evalResult{}, false
}

func (e *evaluator) getConfigOverrideEvalForGCIR(spec configSpec, options *GCIROptions) (*evalResult, bool) {

	if options.Overrides != nil && options.Overrides.DynamicConfigs != nil {
		expOptions, exists := options.Overrides.DynamicConfigs[spec.Name]

		if !exists {
			if options.IncludeLocalOverrides {
				return e.getConfigOverrideEval(spec.Name)
			}
			return &evalResult{}, false
		}

		overrideRule := e.ruleOverrideForClientInitializeResponse(spec, expOptions.GroupName)

		overrideEval := e.overrideEvalForClientInitializeResponse(expOptions.Value, overrideRule)
		if overrideEval != nil {
			return overrideEval, true
		}
	} else {
		if options.IncludeLocalOverrides {
			return e.getConfigOverrideEval(spec.Name)
		}
		return &evalResult{}, false
	}

	return &evalResult{}, false
}

func (e *evaluator) getLayerOverrideEvalForGCIR(spec configSpec, options *GCIROptions) (*evalResult, bool) {
	if options.Overrides != nil && options.Overrides.Layers != nil {
		layerOptions, exists := options.Overrides.Layers[spec.Name]

		if !exists {
			if options.IncludeLocalOverrides {
				return e.getLayerOverrideEval(spec.Name)
			}
			return &evalResult{}, false
		}

		overrideEval := e.overrideEvalForClientInitializeResponse(layerOptions.Value, nil)
		if overrideEval != nil {
			return overrideEval, true
		}
	} else {
		if options.IncludeLocalOverrides {
			return e.getLayerOverrideEval(spec.Name)
		}
		return &evalResult{}, false
	}

	return &evalResult{}, false
}

func (e *evaluator) ruleOverrideForClientInitializeResponse(spec configSpec, groupName string) *configRule {
	if groupName == "" {
		return nil
	}

	for i := range spec.Rules {
		rule := &spec.Rules[i]
		if rule.GroupName == groupName && rule.IsExperimentGroup != nil && *rule.IsExperimentGroup {
			return rule
		}
	}

	return nil
}

func (e *evaluator) overrideEvalForClientInitializeResponse(
	valueOverride map[string]interface{},
	ruleOverride *configRule,
) *evalResult {
	var value map[string]interface{}

	if valueOverride != nil {
		value = valueOverride
	} else if ruleOverride != nil {
		value = ruleOverride.ReturnValueJSON
	}

	if value == nil {
		return nil
	}

	idType := "userid"
	if ruleOverride != nil && ruleOverride.IDType != "" {
		idType = ruleOverride.IDType
	}

	groupName := ""
	if ruleOverride != nil {
		groupName = ruleOverride.GroupName
	}

	return &evalResult{
		Value:              true,
		RuleID:             "override",
		GroupName:          groupName,
		IDType:             idType,
		SecondaryExposures: []SecondaryExposure{},
		JsonValue:          value,
	}
}

// Gets all evaluated values for the given user.
// These values can then be given to a Statsig Client SDK via bootstrapping.
func (e *evaluator) getClientInitializeResponse(
	user User,
	context *evalContext,
	options *GCIROptions,
) ClientInitializeResponse {
	return getClientInitializeResponse(user, e, context, options)
}

func (e *evaluator) cleanExposures(exposures []SecondaryExposure) []SecondaryExposure {
	seen := make(map[string]bool)
	result := make([]SecondaryExposure, 0)
	for _, exposure := range exposures {
		key := fmt.Sprintf("%s|%s|%s", exposure.Gate, exposure.GateValue, exposure.RuleID)
		if _, exists := seen[key]; !exists {
			seen[key] = true
			result = append(result, exposure)
		}
	}
	return result
}

func (e *evaluator) eval(user User, spec configSpec, depth int, context *evalContext) *evalResult {
	if depth > maxRecursiveDepth {
		panic(errors.New("statsig evaluation depth exceeded"))
	}
	var configValue map[string]interface{}
	evalDetails := e.createEvaluationDetails(ReasonNone)
	isDynamicConfig := strings.EqualFold(spec.Type, dynamicConfigType)
	if isDynamicConfig {
		configValue = spec.DefaultValueJSON
	}

	var exposures = make([]SecondaryExposure, 0)
	defaultRuleID := "default"
	var deviceMetadata *DerivedDeviceMetadata
	if spec.Enabled {
		for _, rule := range spec.Rules {
			context.EvalSamplingRate = rule.SamplingRate
			r := e.evalRule(user, rule, depth+1, context)
			if r.Unsupported {
				r.EvaluationDetails = e.createEvaluationDetails(ReasonUnsupported)
				return r
			}
			exposures = e.cleanExposures(append(exposures, r.SecondaryExposures...))
			deviceMetadata = assignDerivedDeviceMetadata(r, deviceMetadata)
			if r.Value {
				delegatedResult := e.evalDelegate(user, rule, exposures, depth+1, context)
				if delegatedResult != nil {
					return delegatedResult
				}

				pass := evalPassPercent(user, rule, spec)
				if isDynamicConfig {
					if pass {
						configValue = rule.ReturnValueJSON
					}
					result := &evalResult{
						Value:                         pass,
						JsonValue:                     configValue,
						RuleID:                        rule.ID,
						IDType:                        spec.IDType,
						GroupName:                     rule.GroupName,
						SecondaryExposures:            exposures,
						UndelegatedSecondaryExposures: exposures,
						EvaluationDetails:             evalDetails,
						DerivedDeviceMetadata:         deviceMetadata,
						SamplingRate:                  rule.SamplingRate,
						ConfigVersion:                 spec.ConfigVersion,
						HasSeenAnalyticalGates:        context.EvalHasSeenAnalyticalGates,
					}
					if rule.IsExperimentGroup != nil {
						result.IsExperimentGroup = rule.IsExperimentGroup
					}
					return result
				} else {
					return &evalResult{
						Value:                  pass,
						RuleID:                 rule.ID,
						IDType:                 spec.IDType,
						GroupName:              rule.GroupName,
						SecondaryExposures:     exposures,
						EvaluationDetails:      evalDetails,
						DerivedDeviceMetadata:  deviceMetadata,
						SamplingRate:           rule.SamplingRate,
						ConfigVersion:          spec.ConfigVersion,
						HasSeenAnalyticalGates: context.EvalHasSeenAnalyticalGates,
					}
				}
			}
		}
	} else {
		defaultRuleID = "disabled"
	}

	if isDynamicConfig {
		return &evalResult{
			Value:                         false,
			JsonValue:                     configValue,
			RuleID:                        defaultRuleID,
			IDType:                        spec.IDType,
			SecondaryExposures:            exposures,
			UndelegatedSecondaryExposures: exposures,
			EvaluationDetails:             evalDetails,
			DerivedDeviceMetadata:         deviceMetadata,
			ConfigVersion:                 spec.ConfigVersion,
			HasSeenAnalyticalGates:        context.EvalHasSeenAnalyticalGates,
		}
	}
	return &evalResult{Value: false, RuleID: defaultRuleID, IDType: spec.IDType,
		SecondaryExposures: exposures, DerivedDeviceMetadata: deviceMetadata, ConfigVersion: spec.ConfigVersion, HasSeenAnalyticalGates: context.EvalHasSeenAnalyticalGates,
	}
}

func (e *evaluator) evalDelegate(user User, rule configRule, exposures []SecondaryExposure, depth int, context *evalContext) *evalResult {
	config, hasConfig := e.store.getDynamicConfig(rule.ConfigDelegate)
	if !hasConfig {
		return nil
	}

	result := e.eval(user, config, depth+1, context)
	result.ConfigDelegate = rule.ConfigDelegate
	result.SecondaryExposures = e.cleanExposures(append(exposures, result.SecondaryExposures...))
	result.UndelegatedSecondaryExposures = exposures
	result.ExplicitParameters = config.ExplicitParameters
	return result
}

func evalPassPercent(user User, rule configRule, spec configSpec) bool {
	ruleSalt := rule.Salt
	if ruleSalt == "" {
		ruleSalt = rule.ID
	}
	if rule.PassPercentage == 0.0 {
		return false
	}
	if rule.PassPercentage == 100.0 {
		return true
	}

	hash := getHashUint64Encoding(spec.Salt + "." + ruleSalt + "." + getUnitID(user, rule.IDType))
	return float64(hash%10000) < (rule.PassPercentage * 100)
}

func getUnitID(user User, idType string) string {
	if idType != "" && !strings.EqualFold(idType, "userid") {
		if val, ok := user.CustomIDs[idType]; ok {
			return val
		}
		if val, ok := user.CustomIDs[strings.ToLower(idType)]; ok {
			return val
		}
		return ""
	}
	return user.UserID
}

func (e *evaluator) evalRule(user User, rule configRule, depth int, context *evalContext) *evalResult {
	var exposures = make([]SecondaryExposure, 0)
	var deviceMetadata *DerivedDeviceMetadata
	var finalResult = &evalResult{Value: true, Unsupported: false}
	for _, cond := range rule.Conditions {
		res := e.evalCondition(user, cond, depth+1, context)
		if !res.Value {
			finalResult.Value = false
		}
		if res.Unsupported {
			finalResult.Unsupported = true
		}
		deviceMetadata = assignDerivedDeviceMetadata(res, deviceMetadata)
		exposures = append(exposures, res.SecondaryExposures...)
	}
	finalResult.SecondaryExposures = exposures
	finalResult.DerivedDeviceMetadata = deviceMetadata
	finalResult.SamplingRate = rule.SamplingRate
	return finalResult
}

func (e *evaluator) evalCondition(user User, cond configCondition, depth int, context *evalContext) *evalResult {
	var value interface{}
	condType := cond.Type
	op := cond.Operator
	var deviceMetadata *DerivedDeviceMetadata

	switch {
	case strings.EqualFold(condType, "public"):
		return &evalResult{Value: true}

	case strings.EqualFold(condType, "fail_gate") || strings.EqualFold(condType, "pass_gate"):
		dependentGateName, ok := cond.TargetValue.(string)
		if !ok {
			return &evalResult{Value: strings.EqualFold(condType, "fail_gate")}
		}
		result := e.evalGateImpl(user, dependentGateName, depth+1, context)
		if result.Unsupported {
			return &evalResult{Unsupported: true}
		}
		allExposures := result.SecondaryExposures
		if !strings.HasPrefix(dependentGateName, "segment:") {
			newExposure := SecondaryExposure{
				Gate:      hashName(context.Hash, dependentGateName),
				GateValue: strconv.FormatBool(result.Value),
				RuleID:    result.RuleID,
			}
			allExposures = append(result.SecondaryExposures, newExposure)
		}

		if context.EvalSamplingRate == nil && !strings.HasPrefix(dependentGateName, "segment:") {
			context.EvalHasSeenAnalyticalGates = true
		}

		if strings.EqualFold(condType, "pass_gate") {
			return &evalResult{Value: result.Value, SecondaryExposures: allExposures, DerivedDeviceMetadata: result.DerivedDeviceMetadata}
		} else {
			return &evalResult{Value: !result.Value, SecondaryExposures: allExposures, DerivedDeviceMetadata: result.DerivedDeviceMetadata}
		}
	case strings.EqualFold(condType, "ip_based"):
		value = getFromUser(user, cond.Field)
		if value == nil || value == "" {
			value = getFromIP(user, cond.Field, e.countryLookup)
		}
	case strings.EqualFold(condType, "ua_based"):
		value = getFromUser(user, cond.Field)
		if value == nil || value == "" {
			deviceMetadata = &DerivedDeviceMetadata{}
			value = getFromUserAgent(user, cond.Field, e.uaParser, deviceMetadata)
		}
	case strings.EqualFold(condType, "user_field"):
		value = getFromUser(user, cond.Field)
	case strings.EqualFold(condType, "environment_field"):
		value = getFromEnvironment(user, cond.Field)
	case strings.EqualFold(condType, "current_time"):
		value = time.Now().Unix() // time in seconds
	case strings.EqualFold(condType, "user_bucket"):
		if salt, ok := cond.AdditionalValues["salt"]; ok {
			value = int64(getHashUint64Encoding(fmt.Sprintf("%s.%s", salt, getUnitID(user, cond.IDType))) % 1000)
		}
	case strings.EqualFold(condType, "unit_id"):
		value = getUnitID(user, cond.IDType)
	case strings.EqualFold(condType, "target_app"):
		if context.ClientKey != "" {
			value = context.TargetAppID
		} else {
			value = e.store.getAppID()
		}
	default:
		return &evalResult{Unsupported: true}
	}

	pass := false
	unsupported := false
	switch {
	case strings.EqualFold(op, "gt"):
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x > y })
	case strings.EqualFold(op, "gte"):
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x >= y })
	case strings.EqualFold(op, "lt"):
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x < y })
	case strings.EqualFold(op, "lte"):
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x <= y })
	case strings.EqualFold(op, "version_gt"):
		pass = compareVersions(value, cond.TargetValue, func(x, y []int64) bool { return compareVersionsHelper(x, y) > 0 })
	case strings.EqualFold(op, "version_gte"):
		pass = compareVersions(value, cond.TargetValue, func(x, y []int64) bool { return compareVersionsHelper(x, y) >= 0 })
	case strings.EqualFold(op, "version_lt"):
		pass = compareVersions(value, cond.TargetValue, func(x, y []int64) bool { return compareVersionsHelper(x, y) < 0 })
	case strings.EqualFold(op, "version_lte"):
		pass = compareVersions(value, cond.TargetValue, func(x, y []int64) bool { return compareVersionsHelper(x, y) <= 0 })
	case strings.EqualFold(op, "version_eq"):
		pass = compareVersions(value, cond.TargetValue, func(x, y []int64) bool { return compareVersionsHelper(x, y) == 0 })
	case strings.EqualFold(op, "version_neq"):
		pass = compareVersions(value, cond.TargetValue, func(x, y []int64) bool { return compareVersionsHelper(x, y) != 0 })

	// one to array operations
	case strings.EqualFold(op, "any"):
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			if cond.UserBucket != nil {
				return lookupUserBucket(value, cond.UserBucket)
			} else {
				return compareStrings(x, y, false, func(s1, s2 string) bool { return strings.EqualFold(s1, s2) })
			}
		})
	case strings.EqualFold(op, "none"):
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			if cond.UserBucket != nil {
				return lookupUserBucket(value, cond.UserBucket)
			} else {
				return compareStrings(x, y, false, func(s1, s2 string) bool { return strings.EqualFold(s1, s2) })
			}
		})
	case strings.EqualFold(op, "any_case_sensitive"):
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})
	case strings.EqualFold(op, "none_case_sensitive"):
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})

	// array to array operations
	case strings.EqualFold(op, "array_contains_any"):
		targetArr, okTarget := cond.TargetValue.([]interface{})
		valArr, okVal := value.([]interface{})
		if !okTarget || !okVal {
			pass = false
		} else {
			pass = arrayContainsAny(targetArr, valArr)
		}
	case strings.EqualFold(op, "array_contains_none"):
		targetArr, okTarget := cond.TargetValue.([]interface{})
		valArr, okVal := value.([]interface{})
		if !okTarget || !okVal {
			pass = false
		} else {
			pass = !arrayContainsAny(targetArr, valArr)
		}
	case strings.EqualFold(op, "array_contains_all"):
		targetArr, okTarget := cond.TargetValue.([]interface{})
		valArr, okVal := value.([]interface{})

		if !okTarget || !okVal {
			pass = false
		} else {
			pass = arrayContainsAll(targetArr, valArr)
		}
	case strings.EqualFold(op, "not_array_contains_all"):
		targetArr, okTarget := cond.TargetValue.([]interface{})
		valArr, okVal := value.([]interface{})

		if !okTarget || !okVal {
			pass = false
		} else {
			pass = !arrayContainsAll(targetArr, valArr)
		}

	// string operations
	case strings.EqualFold(op, "str_starts_with_any"):
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.HasPrefix(s1, s2) })
		})
	case strings.EqualFold(op, "str_ends_with_any"):
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.HasSuffix(s1, s2) })
		})
	case strings.EqualFold(op, "str_contains_any"):
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.Contains(s1, s2) })
		})
	case strings.EqualFold(op, "str_contains_none"):
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.Contains(s1, s2) })
		})
	case strings.EqualFold(op, "str_matches"):
		if cond.TargetValue == nil || value == nil {
			pass = cond.TargetValue == nil && value == nil
		} else {
			matched, _ := regexp.MatchString(castToString(cond.TargetValue), castToString(value))
			pass = matched
		}

	// strict equality
	case strings.EqualFold(op, "eq") || strings.EqualFold(op, "neq"):
		equal := false
		// because certain user values are of string type, which cannot be nil, we should check for both nil and empty string
		if cond.TargetValue == nil {
			equal = value == nil || value == ""
		} else {
			equal = reflect.DeepEqual(value, cond.TargetValue)
		}
		if strings.EqualFold(op, "eq") {
			pass = equal
		} else {
			pass = !equal
		}

	// time
	case strings.EqualFold(op, "before"):
		pass = getTime(value).Before(getTime(cond.TargetValue))
	case strings.EqualFold(op, "after"):
		pass = getTime(value).After(getTime(cond.TargetValue))
	case strings.EqualFold(op, "on"):
		y1, m1, d1 := getTime(value).Date()
		y2, m2, d2 := getTime(cond.TargetValue).Date()
		pass = (y1 == y2 && m1 == m2 && d1 == d2)
	case strings.EqualFold(op, "in_segment_list") || strings.EqualFold(op, "not_in_segment_list"):
		inlist := false
		if reflect.TypeOf(cond.TargetValue).String() == "string" && reflect.TypeOf(value).String() == "string" {
			list := e.store.getIDList(castToString(cond.TargetValue))
			if list != nil && list.ids != nil {
				h := sha256.Sum256([]byte(castToString(value)))
				_, inlist = list.ids.Load(base64.StdEncoding.EncodeToString(h[:])[:8])
			}
		}
		if strings.EqualFold(op, "in_segment_list") {
			pass = inlist
		} else {
			pass = !inlist
		}
	default:
		pass = false
		unsupported = true
	}
	return &evalResult{Value: pass, Unsupported: unsupported, DerivedDeviceMetadata: deviceMetadata}
}

func getFromUser(user User, field string) interface{} {
	var value interface{}
	// 1. Try to get from top level user field first
	switch {
	case strings.EqualFold(field, "userid") || strings.EqualFold(field, "user_id"):
		value = user.UserID
	case strings.EqualFold(field, "email"):
		value = user.Email
	case strings.EqualFold(field, "ip") || strings.EqualFold(field, "ipaddress") || strings.EqualFold(field, "ip_address"):
		value = user.IpAddress
	case strings.EqualFold(field, "useragent") || strings.EqualFold(field, "user_agent"):
		if user.UserAgent != "" { // UserAgent cannot be empty string
			value = user.UserAgent
		}
	case strings.EqualFold(field, "country"):
		value = user.Country
	case strings.EqualFold(field, "locale"):
		value = user.Locale
	case strings.EqualFold(field, "appversion") || strings.EqualFold(field, "app_version"):
		value = user.AppVersion
	}

	// 2. Check custom user attributes and then private attributes next
	if value == "" || value == nil {
		if customValue, ok := user.Custom[field]; ok {
			value = customValue
		} else if customValue, ok := user.Custom[strings.ToLower(field)]; ok {
			value = customValue
		} else if privateValue, ok := user.PrivateAttributes[field]; ok {
			value = privateValue
		} else if privateValue, ok := user.PrivateAttributes[strings.ToLower(field)]; ok {
			value = privateValue
		}
	}

	return value
}

func getFromEnvironment(user User, field string) string {
	var value string
	if val, ok := user.StatsigEnvironment[field]; ok {
		value = val
	}
	if val, ok := user.StatsigEnvironment[strings.ToLower(field)]; ok {
		value = val
	}
	return value
}

func getFromUserAgent(user User, field string, parser *uaParser, deviceMetadata *DerivedDeviceMetadata) string {
	ua := getFromUser(user, "useragent")
	uaStr, ok := ua.(string)
	if !ok {
		return ""
	}
	client := parser.parse(uaStr)
	if client == nil {
		return ""
	}
	switch {
	case strings.EqualFold(field, "os_name") || strings.EqualFold(field, "osname"):
		if deviceMetadata != nil {
			deviceMetadata.OsName = client.Os.Family
		}
		return client.Os.Family
	case strings.EqualFold(field, "os_version") || strings.EqualFold(field, "osversion"):
		osVersion := strings.Join(removeEmptyStrings([]string{client.Os.Major, client.Os.Minor, client.Os.Patch, client.Os.PatchMinor}), ".")
		if deviceMetadata != nil {
			deviceMetadata.OsVersion = osVersion
		}
		return osVersion
	case strings.EqualFold(field, "browser_name") || strings.EqualFold(field, "browsername"):
		if deviceMetadata != nil {
			deviceMetadata.BrowserName = client.UserAgent.Family
		}
		return client.UserAgent.Family
	case strings.EqualFold(field, "browser_version") || strings.EqualFold(field, "browserversion"):
		browserVersion := strings.Join(removeEmptyStrings([]string{client.UserAgent.Major, client.UserAgent.Minor, client.UserAgent.Patch}), ".")
		if deviceMetadata != nil {
			deviceMetadata.BrowserVersion = browserVersion
		}
		return browserVersion
	}
	return ""
}

func getFromIP(user User, field string, lookup *countryLookup) string {
	if !strings.EqualFold(field, "country") {
		return ""
	}

	ip := getFromUser(user, "ip")
	if ipStr, ok := ip.(string); ok {
		if res, lookupOK := lookup.lookupIp(ipStr); lookupOK {
			return res
		}
	}

	return ""
}

func removeEmptyStrings(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func castToString(a interface{}) string {
	asString, ok := a.(string)
	if !ok {
		return ""
	}
	return asString
}

func convertToString(a interface{}) string {
	if a == nil {
		return ""
	}
	if asString, ok := a.(string); ok {
		return asString
	}
	aVal := reflect.ValueOf(a)
	switch aVal.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(aVal.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(aVal.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(aVal.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(aVal.Bool())
	case reflect.String:
		return fmt.Sprintf("%v", a)
	case reflect.Slice, reflect.Array:
		var result []string
		for i := 0; i < aVal.Len(); i++ {
			result = append(result, fmt.Sprintf("%v", aVal.Index(i).Interface()))
		}
		return strings.Join(result, ",")
	}

	return fmt.Sprintf("%v", a)
}

func compareNumbers(a, b interface{}, fun func(x, y float64) bool) bool {
	numA, okA := getNumericValue(a)
	numB, okB := getNumericValue(b)
	if !okA || !okB {
		return false
	}
	return fun(numA, numB)
}

func lookupUserBucket(val interface{}, lookup map[int64]bool) bool {
	if valInt, ok := val.(int64); ok {
		_, pass := lookup[valInt]
		return pass
	}
	return false
}

func compareStrings(s1 interface{}, s2 interface{}, ignoreCase bool, fun func(x, y string) bool) bool {
	var str1, str2 string
	if s1 == nil || s2 == nil {
		return false
	}
	str1 = convertToString(s1)
	str2 = convertToString(s2)

	if ignoreCase {
		return fun(strings.ToLower(str1), strings.ToLower(str2))
	}
	return fun(str1, str2)
}

func convertVersionStringToParts(version string) ([]int64, error) {
	stringParts := strings.Split(version, ".")
	numParts := make([]int64, len(stringParts))
	for i := range stringParts {
		n1, e := strconv.ParseInt(stringParts[i], 10, 64)
		if e != nil {
			return numParts, e
		}
		numParts[i] = n1
	}
	return numParts, nil
}

func compareVersionsHelper(v1 []int64, v2 []int64) int {
	i := 0
	v1len := len(v1)
	v2len := len(v2)
	for i < maxInt(v1len, v2len) {
		var n1 int64
		if i >= v1len {
			n1 = 0
		} else {
			n1 = v1[i]
		}
		var n2 int64
		if i >= v2len {
			n2 = 0
		} else {
			n2 = v2[i]
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
		i++
	}
	return 0
}

func compareVersions(a, b interface{}, fun func(x, y []int64) bool) bool {
	strA, okA := a.(string)
	strB, okB := b.(string)
	if !okA || !okB {
		return false
	}
	v1 := strings.Split(strA, "-")[0]
	v2 := strings.Split(strB, "-")[0]
	if len(v1) == 0 || len(v2) == 0 {
		return false
	}

	v1Parts, e1 := convertVersionStringToParts(v1)
	v2Parts, e2 := convertVersionStringToParts(v2)
	if e1 != nil || e2 != nil {
		return false
	}
	return fun(v1Parts, v2Parts)
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func arrayAny(arr interface{}, val interface{}, fun func(x, y interface{}) bool) bool {
	if array, ok := arr.([]interface{}); ok {
		for _, arrVal := range array {
			if fun(val, arrVal) {
				return true
			}
		}
	}
	return false
}

func arrayContainsAll(target []interface{}, value []interface{}) bool {
	valueSet := make(map[string]struct{})
	for _, item := range value {
		valStr := convertToString(item)
		valueSet[valStr] = struct{}{}
	}

	for _, t := range target {
		strTarget := convertToString(t)
		_, strExists := valueSet[strTarget]
		if !strExists {
			return false
		}
	}
	return true
}

func arrayContainsAny(target []interface{}, value []interface{}) bool {
	valueSet := make(map[string]struct{})
	for _, item := range value {
		valStr := convertToString(item)
		valueSet[valStr] = struct{}{}
	}

	for _, t := range target {
		strTarget := convertToString(t)
		_, strExists := valueSet[strTarget]
		if strExists {
			return true
		}
	}
	return false
}

func getTime(a interface{}) time.Time {
	switch v := a.(type) {
	case float64, int64, int32, int:
		t_sec := time.Unix(getUnixTimestamp(v), 0)
		if t_sec.Year() > time.Now().Year()+100 {
			return time.Unix(getUnixTimestamp(v)/1000, 0)
		}
		return t_sec
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t
		}
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}
		}
		t_sec := time.Unix(getUnixTimestamp(vInt), 0)
		if t_sec.Year() > time.Now().Year()+100 {
			return time.Unix(getUnixTimestamp(vInt)/1000, 0)
		}
		return t_sec
	}
	return time.Time{}
}

func getUnixTimestamp(v interface{}) int64 {
	switch v := v.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	}
	return 0
}

func assignDerivedDeviceMetadata(res *evalResult, deviceMetadata *DerivedDeviceMetadata) *DerivedDeviceMetadata {
	if res.DerivedDeviceMetadata != nil {
		if deviceMetadata == nil {
			deviceMetadata = &DerivedDeviceMetadata{}
		}
		deviceMetadata.OsName = res.DerivedDeviceMetadata.OsName
		deviceMetadata.OsVersion = res.DerivedDeviceMetadata.OsVersion
		deviceMetadata.BrowserName = res.DerivedDeviceMetadata.BrowserName
		deviceMetadata.BrowserVersion = res.DerivedDeviceMetadata.BrowserVersion
	}
	return deviceMetadata
}

func (e *evaluator) applyCMABSampling(cmab *configSpec, cmabConfig map[string]cmabGroupConfig, exposures []SecondaryExposure) *evalResult {
	totalRecords := 0.0
	for _, group := range cmab.Groups {
		groupID := group.ID
		config, exists := cmabConfig[groupID]
		curCount := 1.0
		if exists {
			curCount += float64(config.Records)
		}
		totalRecords += 1.0 / curCount
	}

	sum := 0.0
	value := rand.Float64()
	for _, group := range cmab.Groups {
		groupID := group.ID
		config, exists := cmabConfig[groupID]
		curCount := 1.0
		if exists {
			curCount += float64(config.Records)
		}
		sum += 1.0 / curCount / totalRecords
		if value < sum {
			result := &evalResult{
				JsonValue:          group.ParameterValuesJSON,
				RuleID:             group.ID + ":explore",
				GroupName:          group.Name,
				SecondaryExposures: exposures,
				EvaluationDetails:  e.createEvaluationDetails(ReasonNone),
			}
			isExperimentGroup := true
			result.IsExperimentGroup = &isExperimentGroup
			return result
		}
	}
	return nil
}

func (e *evaluator) applyCMABBestGroup(cmab *configSpec, cmabConfig map[string]cmabGroupConfig, user User, exposures []SecondaryExposure) *evalResult {
	higherBetter := cmab.HigherIsBetter
	var bestScore float64
	if higherBetter {
		bestScore = -1000000000
	} else {
		bestScore = 1000000000
	}
	hasScore := false
	var bestGroup cmabGroup

	for _, group := range cmab.Groups {
		groupID := group.ID
		config, exists := cmabConfig[groupID]
		if !exists {
			continue
		}

		weightsNumerical := config.WeightsNumerical
		weightsCategorical := config.WeightsCategorical
		if len(weightsNumerical) == 0 && len(weightsCategorical) == 0 {
			continue
		}

		score := 0.0
		score += config.Alpha + config.Intercept

		for key, weight := range weightsCategorical {
			untypedVal := getFromUser(user, key)
			value, ok := untypedVal.(string)
			if !ok || value == "" {
				continue
			}
			if w, exists := weight[value]; exists {
				score += w
			}
		}

		for key, weight := range weightsNumerical {
			untypedVal := getFromUser(user, key)
			value, ok := untypedVal.(float64)
			if ok {
				score += value * weight
			}
		}

		if !hasScore || (higherBetter && score > bestScore) || (!higherBetter && score < bestScore) {
			bestScore = score
			bestGroup = group
			hasScore = true
		}
	}

	if !hasScore {
		randIndex := rand.Intn(len(cmab.Groups))
		bestGroup = cmab.Groups[randIndex]
	}

	isExperimentGroup := true
	return &evalResult{
		Value:              true,
		JsonValue:          bestGroup.ParameterValuesJSON,
		RuleID:             bestGroup.ID,
		GroupName:          bestGroup.Name,
		IsExperimentGroup:  &isExperimentGroup,
		SecondaryExposures: exposures,
	}
}

func (e *evaluator) finalizeCMABEvalResult(cmab *configSpec, result *evalResult, didPass bool) {
	result.Value = didPass
	result.IDType = cmab.IDType
	result.TargetAppIDs = cmab.TargetAppIDs
	result.ConfigVersion = cmab.ConfigVersion
}
