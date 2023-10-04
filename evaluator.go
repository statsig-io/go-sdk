package statsig

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/statsig-io/ip3country-go/pkg/countrylookup"
	"github.com/ua-parser/uap-go/uaparser"
)

type evaluator struct {
	store           *store
	gateOverrides   map[string]bool
	configOverrides map[string]map[string]interface{}
	layerOverrides  map[string]map[string]interface{}
	countryLookup   *countrylookup.CountryLookup
	uaParser        *uaparser.Parser
	mu              sync.RWMutex
}

type evalResult struct {
	Pass                          bool
	ConfigValue                   DynamicConfig
	FetchFromServer               bool
	Id                            string
	SecondaryExposures            []map[string]string
	UndelegatedSecondaryExposures []map[string]string
	ConfigDelegate                string
	ExplicitParameters            map[string]bool
	EvaluationDetails             *evaluationDetails
	IsExperimentGroup             *bool
}

const dynamicConfigType = "dynamic_config"
const maxRecursiveDepth = 300

func newEvaluator(
	transport *transport,
	errorBoundary *errorBoundary,
	options *Options,
	diagnostics *diagnostics,
) *evaluator {
	store := newStore(transport, errorBoundary, options, diagnostics)
	parser := uaparser.NewFromSaved()
	countryLookup := countrylookup.New()
	defer func() {
		if err := recover(); err != nil {
			errorBoundary.logException(toError(err))
			global.Logger().LogError(err)
		}
	}()

	return &evaluator{
		store:           store,
		countryLookup:   countryLookup,
		uaParser:        parser,
		gateOverrides:   make(map[string]bool),
		configOverrides: make(map[string]map[string]interface{}),
		layerOverrides:  make(map[string]map[string]interface{}),
	}
}

func (e *evaluator) shutdown() {
	if e.store.dataAdapter != nil {
		e.store.dataAdapter.Shutdown()
	}
	e.store.stopPolling()
}

func (e *evaluator) createEvaluationDetails(reason evaluationReason) *evaluationDetails {
	e.store.mu.RLock()
	defer e.store.mu.RUnlock()
	return newEvaluationDetails(reason, e.store.lastSyncTime, e.store.initialSyncTime)
}

func (e *evaluator) checkGate(user User, gateName string) *evalResult {
	return e.evalGate(user, gateName, 0)
}

func (e *evaluator) evalGate(user User, gateName string, depth int) *evalResult {
	if gateOverride, hasOverride := e.getGateOverride(gateName); hasOverride {
		evalDetails := e.createEvaluationDetails(reasonLocalOverride)
		return &evalResult{
			Pass:               gateOverride,
			Id:                 "override",
			EvaluationDetails:  evalDetails,
			SecondaryExposures: make([]map[string]string, 0),
		}
	}
	if gate, hasGate := e.store.getGate(gateName); hasGate {
		return e.eval(user, gate, depth+1)
	}
	emptyEvalResult := new(evalResult)
	emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(reasonUnrecognized)
	emptyEvalResult.SecondaryExposures = make([]map[string]string, 0)
	return emptyEvalResult
}

func (e *evaluator) getConfig(user User, configName string) *evalResult {
	return e.evalConfig(user, configName, 0)
}

func (e *evaluator) evalConfig(user User, configName string, depth int) *evalResult {
	if configOverride, hasOverride := e.getConfigOverride(configName); hasOverride {
		evalDetails := e.createEvaluationDetails(reasonLocalOverride)
		return &evalResult{
			Pass:               true,
			ConfigValue:        *NewConfig(configName, configOverride, "override"),
			Id:                 "override",
			EvaluationDetails:  evalDetails,
			SecondaryExposures: make([]map[string]string, 0),
		}
	}
	if config, hasConfig := e.store.getDynamicConfig(configName); hasConfig {
		return e.eval(user, config, depth+1)
	}
	emptyEvalResult := new(evalResult)
	emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(reasonUnrecognized)
	emptyEvalResult.SecondaryExposures = make([]map[string]string, 0)
	return emptyEvalResult
}

func (e *evaluator) getLayer(user User, name string) *evalResult {
	return e.evalLayer(user, name, 0)
}

func (e *evaluator) evalLayer(user User, name string, depth int) *evalResult {
	if layerOverride, hasOverride := e.getLayerOverride(name); hasOverride {
		evalDetails := e.createEvaluationDetails(reasonLocalOverride)
		return &evalResult{
			Pass:               true,
			ConfigValue:        *NewConfig(name, layerOverride, "override"),
			Id:                 "override",
			EvaluationDetails:  evalDetails,
			SecondaryExposures: make([]map[string]string, 0),
		}
	}
	if config, hasConfig := e.store.getLayerConfig(name); hasConfig {
		return e.eval(user, config, depth+1)
	}
	emptyEvalResult := new(evalResult)
	emptyEvalResult.EvaluationDetails = e.createEvaluationDetails(reasonUnrecognized)
	emptyEvalResult.SecondaryExposures = make([]map[string]string, 0)
	return emptyEvalResult
}

func (e *evaluator) getGateOverride(name string) (bool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	gate, ok := e.gateOverrides[name]
	return gate, ok
}

func (e *evaluator) getConfigOverride(name string) (map[string]interface{}, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	config, ok := e.configOverrides[name]
	return config, ok
}

func (e *evaluator) getLayerOverride(name string) (map[string]interface{}, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	layer, ok := e.layerOverrides[name]
	return layer, ok
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

// Gets all evaluated values for the given user.
// These values can then be given to a Statsig Client SDK via bootstrapping.
func (e *evaluator) getClientInitializeResponse(user User, clientKey string) ClientInitializeResponse {
	return getClientInitializeResponse(user, e.store, e.eval, clientKey)
}

func (e *evaluator) eval(user User, spec configSpec, depth int) *evalResult {
	if depth > maxRecursiveDepth {
		panic(errors.New("Statsig Evaluation Depth Exceeded"))
	}
	var configValue map[string]interface{}
	e.store.mu.RLock()
	reason := e.store.initReason
	e.store.mu.RUnlock()
	evalDetails := e.createEvaluationDetails(reason)
	isDynamicConfig := strings.ToLower(spec.Type) == dynamicConfigType
	if isDynamicConfig {
		err := json.Unmarshal(spec.DefaultValue, &configValue)
		if err != nil {
			configValue = make(map[string]interface{})
		}
	}

	var exposures = make([]map[string]string, 0)
	defaultRuleID := "default"
	if spec.Enabled {
		for _, rule := range spec.Rules {
			r := e.evalRule(user, rule, depth+1)
			if r.FetchFromServer {
				return r
			}
			exposures = append(exposures, r.SecondaryExposures...)
			if r.Pass {

				delegatedResult := e.evalDelegate(user, rule, exposures, depth+1)
				if delegatedResult != nil {
					return delegatedResult
				}

				pass := evalPassPercent(user, rule, spec)
				if isDynamicConfig {
					if pass {
						var ruleConfigValue map[string]interface{}
						err := json.Unmarshal(rule.ReturnValue, &ruleConfigValue)
						if err != nil {
							ruleConfigValue = make(map[string]interface{})
						}
						configValue = ruleConfigValue
					}
					result := &evalResult{
						Pass:                          pass,
						ConfigValue:                   *NewConfig(spec.Name, configValue, rule.ID),
						Id:                            rule.ID,
						SecondaryExposures:            exposures,
						UndelegatedSecondaryExposures: exposures,
						EvaluationDetails:             evalDetails,
					}
					if rule.IsExperimentGroup != nil {
						result.IsExperimentGroup = rule.IsExperimentGroup
					}
					return result
				} else {
					return &evalResult{
						Pass:               pass,
						Id:                 rule.ID,
						SecondaryExposures: exposures,
						EvaluationDetails:  evalDetails,
					}
				}
			}
		}
	} else {
		defaultRuleID = "disabled"
	}

	if isDynamicConfig {
		return &evalResult{
			Pass:                          false,
			ConfigValue:                   *NewConfig(spec.Name, configValue, defaultRuleID),
			Id:                            defaultRuleID,
			SecondaryExposures:            exposures,
			UndelegatedSecondaryExposures: exposures,
			EvaluationDetails:             evalDetails,
		}
	}
	return &evalResult{Pass: false, Id: defaultRuleID, SecondaryExposures: exposures}
}

func (e *evaluator) evalDelegate(user User, rule configRule, exposures []map[string]string, depth int) *evalResult {
	config, hasConfig := e.store.getDynamicConfig(rule.ConfigDelegate)
	if !hasConfig {
		return nil
	}

	result := e.eval(user, config, depth+1)
	result.ConfigDelegate = rule.ConfigDelegate
	result.SecondaryExposures = append(exposures, result.SecondaryExposures...)
	result.UndelegatedSecondaryExposures = exposures

	explicitParams := map[string]bool{}
	for _, s := range config.ExplicitParameters {
		explicitParams[s] = true
	}
	result.ExplicitParameters = explicitParams
	return result
}

func evalPassPercent(user User, rule configRule, spec configSpec) bool {
	ruleSalt := rule.Salt
	if ruleSalt == "" {
		ruleSalt = rule.ID
	}
	hash := getHashUint64Encoding(spec.Salt + "." + ruleSalt + "." + getUnitID(user, rule.IDType))

	return float64(hash%10000) < (rule.PassPercentage * 100)
}

func getUnitID(user User, idType string) string {
	if idType != "" && strings.ToLower(idType) != "userid" {
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

func (e *evaluator) evalRule(user User, rule configRule, depth int) *evalResult {
	var exposures = make([]map[string]string, 0)
	var finalResult = &evalResult{Pass: true, FetchFromServer: false}
	for _, cond := range rule.Conditions {
		res := e.evalCondition(user, cond, depth+1)
		if !res.Pass {
			finalResult.Pass = false
		}
		if res.FetchFromServer {
			finalResult.FetchFromServer = true
		}
		exposures = append(exposures, res.SecondaryExposures...)
	}
	finalResult.SecondaryExposures = exposures
	return finalResult
}

func (e *evaluator) evalCondition(user User, cond configCondition, depth int) *evalResult {
	var value interface{}
	condType := strings.ToLower(cond.Type)
	op := strings.ToLower(cond.Operator)
	switch condType {
	case "public":
		return &evalResult{Pass: true}
	case "fail_gate", "pass_gate":
		dependentGateName, ok := cond.TargetValue.(string)
		if !ok {
			return &evalResult{Pass: false}
		}
		result := e.evalGate(user, dependentGateName, depth+1)
		if result.FetchFromServer {
			return &evalResult{FetchFromServer: true}
		}
		newExposure := map[string]string{
			"gate":      dependentGateName,
			"gateValue": strconv.FormatBool(result.Pass),
			"ruleID":    result.Id,
		}
		allExposures := append(result.SecondaryExposures, newExposure)
		if condType == "pass_gate" {
			return &evalResult{Pass: result.Pass, SecondaryExposures: allExposures}
		} else {
			return &evalResult{Pass: !result.Pass, SecondaryExposures: allExposures}
		}
	case "ip_based":
		value = getFromUser(user, cond.Field)
		if value == nil || value == "" {
			value = getFromIP(user, cond.Field, e.countryLookup)
		}
	case "ua_based":
		value = getFromUser(user, cond.Field)
		if value == nil || value == "" {
			value = getFromUserAgent(user, cond.Field, e.uaParser)
		}
	case "user_field":
		value = getFromUser(user, cond.Field)
	case "environment_field":
		value = getFromEnvironment(user, cond.Field)
	case "current_time":
		value = time.Now().Unix() // time in seconds
	case "user_bucket":
		if salt, ok := cond.AdditionalValues["salt"]; ok {
			value = int64(getHashUint64Encoding(fmt.Sprintf("%s.%s", salt, getUnitID(user, cond.IDType))) % 1000)
		}
	case "unit_id":
		value = getUnitID(user, cond.IDType)
	default:
		return &evalResult{FetchFromServer: true}
	}

	pass := false
	server := false
	switch op {
	case "gt":
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x > y })
	case "gte":
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x >= y })
	case "lt":
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x < y })
	case "lte":
		pass = compareNumbers(value, cond.TargetValue, func(x, y float64) bool { return x <= y })
	case "version_gt":
		pass = compareVersions(value, cond.TargetValue, func(x, y string) bool { return compareVersionsHelper(x, y) > 0 })
	case "version_gte":
		pass = compareVersions(value, cond.TargetValue, func(x, y string) bool { return compareVersionsHelper(x, y) >= 0 })
	case "version_lt":
		pass = compareVersions(value, cond.TargetValue, func(x, y string) bool { return compareVersionsHelper(x, y) < 0 })
	case "version_lte":
		pass = compareVersions(value, cond.TargetValue, func(x, y string) bool { return compareVersionsHelper(x, y) <= 0 })
	case "version_eq":
		pass = compareVersions(value, cond.TargetValue, func(x, y string) bool { return compareVersionsHelper(x, y) == 0 })
	case "version_neq":
		pass = compareVersions(value, cond.TargetValue, func(x, y string) bool { return compareVersionsHelper(x, y) != 0 })

	// array operations
	case "any":
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return s1 == s2 })
		})
	case "none":
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return s1 == s2 })
		})
	case "any_case_sensitive":
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})
	case "none_case_sensitive":
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})

	// string operations
	case "str_starts_with_any":
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.HasPrefix(s1, s2) })
		})
	case "str_ends_with_any":
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.HasSuffix(s1, s2) })
		})
	case "str_contains_any":
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.Contains(s1, s2) })
		})
	case "str_contains_none":
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compareStrings(x, y, true, func(s1, s2 string) bool { return strings.Contains(s1, s2) })
		})
	case "str_matches":
		if cond.TargetValue == nil || value == nil {
			pass = cond.TargetValue == nil && value == nil
		} else {
			matched, _ := regexp.MatchString(toString(cond.TargetValue), toString(value))
			pass = matched
		}

	// strict equality
	case "eq", "neq":
		equal := false
		// because certain user values are of string type, which cannot be nil, we should check for both nil and empty string
		if cond.TargetValue == nil {
			equal = value == nil || value == ""
		} else {
			equal = reflect.DeepEqual(value, cond.TargetValue)
		}
		if op == "eq" {
			pass = equal
		} else {
			pass = !equal
		}

	// time
	case "before":
		pass = getTime(value).Before(getTime(cond.TargetValue))
	case "after":
		pass = getTime(value).After(getTime(cond.TargetValue))
	case "on":
		y1, m1, d1 := getTime(value).Date()
		y2, m2, d2 := getTime(cond.TargetValue).Date()
		pass = (y1 == y2 && m1 == m2 && d1 == d2)
	case "in_segment_list", "not_in_segment_list":
		inlist := false
		if reflect.TypeOf(cond.TargetValue).String() == "string" && reflect.TypeOf(value).String() == "string" {
			list := e.store.getIDList(toString(cond.TargetValue))
			if list != nil {
				h := sha256.Sum256([]byte(toString(value)))
				_, inlist = list.ids.Load(base64.StdEncoding.EncodeToString(h[:])[:8])
			}
		}
		if op == "in_segment_list" {
			pass = inlist
		} else {
			pass = !inlist
		}
	default:
		pass = false
		server = true
	}
	return &evalResult{Pass: pass, FetchFromServer: server}
}

func getFromUser(user User, field string) interface{} {
	var value interface{}
	// 1. Try to get from top level user field first
	switch strings.ToLower(field) {
	case "userid", "user_id":
		value = user.UserID
	case "email":
		value = user.Email
	case "ip", "ipaddress", "ip_address":
		value = user.IpAddress
	case "useragent", "user_agent":
		if user.UserAgent != "" { // UserAgent cannot be empty string
			value = user.UserAgent
		}
	case "country":
		value = user.Country
	case "locale":
		value = user.Locale
	case "appversion", "app_version":
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

func getFromUserAgent(user User, field string, parser *uaparser.Parser) string {
	ua := getFromUser(user, "useragent")
	uaStr, ok := ua.(string)
	if !ok {
		return ""
	}
	client := parser.Parse(uaStr)
	switch strings.ToLower(field) {
	case "os_name", "osname":
		return client.Os.Family
	case "os_version", "osversion":
		return strings.Join(removeEmptyStrings([]string{client.Os.Major, client.Os.Minor, client.Os.Patch, client.Os.PatchMinor}), ".")
	case "browser_name", "browsername":
		return client.UserAgent.Family
	case "browser_version", "browserversion":
		return strings.Join(removeEmptyStrings([]string{client.UserAgent.Major, client.UserAgent.Minor, client.UserAgent.Patch}), ".")
	}
	return ""
}

func getFromIP(user User, field string, lookup *countrylookup.CountryLookup) string {
	if strings.ToLower(field) != "country" {
		return ""
	}

	ip := getFromUser(user, "ip")
	if ipStr, ok := ip.(string); ok {
		if res, lookupOK := lookup.LookupIp(ipStr); lookupOK {
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

func getNumericValue(a interface{}) (float64, bool) {
	switch a := a.(type) {
	case int:
		return float64(a), true
	case int32:
		return float64(a), true
	case int64:
		return float64(a), true
	case uint64:
		return float64(a), true
	case float32:
		return float64(a), true
	case float64:
		return a, true
	case string:
		f, err := strconv.ParseFloat(a, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func toString(a interface{}) string {
	asString, ok := a.(string)
	if !ok {
		return ""
	}
	return asString
}

func compareNumbers(a, b interface{}, fun func(x, y float64) bool) bool {
	numA, okA := getNumericValue(a)
	numB, okB := getNumericValue(b)
	if !okA || !okB {
		return false
	}
	return fun(numA, numB)
}

func compareStrings(s1 interface{}, s2 interface{}, ignoreCase bool, fun func(x, y string) bool) bool {
	var str1, str2 string
	if s1 == nil || s2 == nil {
		return false
	}
	if reflect.TypeOf(s1).Kind() == reflect.String {
		str1 = toString(s1)
	} else {
		str1 = fmt.Sprintf("%v", s1)
	}
	if reflect.TypeOf(s2).Kind() == reflect.String {
		str2 = toString(s2)
	} else {
		str2 = fmt.Sprintf("%v", s2)
	}

	if ignoreCase {
		return fun(strings.ToLower(str1), strings.ToLower(str2))
	}
	return fun(str1, str2)
}

func compareVersionsHelper(v1 string, v2 string) int {
	i := 0
	v1Parts := strings.Split(v1, ".")
	v1len := len(v1Parts)
	v2Parts := strings.Split(v2, ".")
	v2len := len(v2Parts)
	for i < maxInt(v1len, v2len) {
		var p1 string
		if i >= v1len {
			p1 = "0"
		} else {
			p1 = v1Parts[i]
		}
		var p2 string
		if i >= v2len {
			p2 = "0"
		} else {
			p2 = v2Parts[i]
		}

		n1, _ := strconv.ParseInt(p1, 10, 64)
		n2, _ := strconv.ParseInt(p2, 10, 64)
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

func compareVersions(a, b interface{}, fun func(x, y string) bool) bool {
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
	return fun(v1, v2)
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
