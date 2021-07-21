package evaluation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"statsig/internal/net"
	"statsig/pkg/types"
	"strconv"
	"strings"
	"time"

	"github.com/statsig-io/ip3country-go/pkg/countrylookup"
	"github.com/ua-parser/uap-go/uaparser"
)

type Evaluator struct {
	store         *Store
	countryLookup *countrylookup.CountryLookup
	uaParser      *uaparser.Parser
}

type EvalResult struct {
	Pass            bool
	ConfigValue     types.DynamicConfig
	FetchFromServer bool
	Id              string
}

var dynamicConfigType = "dynamic_config"

func New(net *net.Net) *Evaluator {
	store := initStore(net)
	parser := uaparser.NewFromSaved()
	countryLookup := countrylookup.New()
	defer func() {
		if err := recover(); err != nil {
			// TODO: log here
			fmt.Println(err)
		}
	}()

	return &Evaluator{
		store:         store,
		countryLookup: countryLookup,
		uaParser:      parser,
	}
}

func (e *Evaluator) Stop() {
	e.store.StopPolling()
}

func (e *Evaluator) CheckGate(user types.StatsigUser, gateName string) *EvalResult {
	if gate, hasGate := e.store.FeatureGates[gateName]; hasGate {
		return e.eval(user, gate)
	}
	return new(EvalResult)
}

func (e *Evaluator) GetConfig(user types.StatsigUser, configName string) *EvalResult {
	if config, hasConfig := e.store.DynamicConfigs[configName]; hasConfig {
		return e.eval(user, config)
	}
	return new(EvalResult)
}

func (e *Evaluator) eval(user types.StatsigUser, spec ConfigSpec) *EvalResult {
	var configValue map[string]interface{}
	isDynamicConfig := strings.ToLower(spec.Type) == dynamicConfigType
	if isDynamicConfig {
		err := json.Unmarshal(spec.DefaultValue, &configValue)
		if err != nil {
			configValue = make(map[string]interface{})
		}
	}

	if spec.Enabled {
		for _, rule := range spec.Rules {
			r := e.evalRule(user, rule)
			if r.FetchFromServer {
				return r
			}
			if r.Pass {
				pass := evalPassPercent(user, rule, spec.Salt)
				if isDynamicConfig {
					if pass {
						err := json.Unmarshal(rule.ReturnValue, &configValue)
						if err != nil {
							configValue = make(map[string]interface{})
						}
					}
					return &EvalResult{
						Pass:        pass,
						ConfigValue: *types.NewConfig(spec.Name, configValue, rule.ID),
						Id:          rule.ID}
				} else {
					return &EvalResult{Pass: pass, Id: rule.ID}
				}
			}
		}
	}

	if isDynamicConfig {
		return &EvalResult{
			Pass:        false,
			ConfigValue: *types.NewConfig(spec.Name, configValue, "default"),
			Id:          "default"}
	}
	return &EvalResult{Pass: false, Id: "default"}
}

func evalPassPercent(user types.StatsigUser, rule ConfigRule, salt string) bool {
	hash := getHash(salt + "." + rule.ID + "." + user.UserID)
	return hash%10000 < (uint64(rule.PassPercentage) * 100)
}

func (e *Evaluator) evalRule(user types.StatsigUser, rule ConfigRule) *EvalResult {
	for _, cond := range rule.Conditions {
		res := e.evalCondition(user, cond)
		if !res.Pass || res.FetchFromServer {
			return res
		}
	}
	return &EvalResult{Pass: true, FetchFromServer: false}
}

func (e *Evaluator) evalCondition(user types.StatsigUser, cond ConfigCondition) *EvalResult {
	// TODO: add all cond evaluations
	var value interface{}
	switch cond.Type {
	case "public":
		return &EvalResult{Pass: true}
	case "fail_gate":
	case "pass_gate":
		dependentGateName, ok := cond.TargetValue.(string)
		if !ok {
			return &EvalResult{Pass: false}
		}
		result := e.CheckGate(user, dependentGateName)
		if result.FetchFromServer {
			return &EvalResult{FetchFromServer: true}
		}
		if cond.Type == "pass_gate" {
			return &EvalResult{Pass: result.Pass}
		} else {
			return &EvalResult{Pass: !result.Pass}
		}
	case "ip_based":
		// TODO: ip3 country
		value = getFromUser(user, cond.Field)
		if value == nil {
			value = getFromIP(user, cond.Field, e.countryLookup)
		}
	case "ua_based":
		value = getFromUser(user, cond.Field)
		if value == nil {
			value = getFromUserAgent(user, cond.Field, e.uaParser)
		}
	case "user_field":
		value = getFromUser(user, cond.Field)
	case "environment_field":
		// TODO: after Tore adds environment
	case "current_time":
		value = time.Now().Unix() // time in seconds
	case "user_bucket":
		if salt, ok := cond.AdditionalValues["salt"]; ok {
			value = getHash(fmt.Sprintf("%s.%s", salt, user.UserID)) % 1000
		}
	default:
		return &EvalResult{FetchFromServer: true}
	}

	if value == nil {
		return &EvalResult{Pass: false}
	}

	pass := false
	server := false
	switch cond.Operator {
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
			return compare(x, y, true)
		})
	case "none":
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compare(x, y, true)
		})
	case "any_case_sensitive":
		pass = arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compare(x, y, false)
		})
	case "none_case_sensitive":
		pass = !arrayAny(cond.TargetValue, value, func(x, y interface{}) bool {
			return compare(x, y, false)
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
	case "str_matches":
		matched, _ := regexp.MatchString(cond.TargetValue.(string), value.(string))
		pass = matched

	// strict equality
	case "eq":
		pass = reflect.DeepEqual(value, cond.TargetValue)
	case "neq":
		pass = !reflect.DeepEqual(value, cond.TargetValue)

	// time
	case "before":
		pass = getTime(value).Before(getTime(cond.TargetValue))
	case "after":
		pass = getTime(value).After(getTime(cond.TargetValue))
	case "on":
		y1, m1, d1 := getTime(value).Date()
		y2, m2, d2 := getTime(cond.TargetValue).Date()
		pass = (y1 == y2 && m1 == m2 && d1 == d2)
	default:
		pass = false
		server = true
	}
	return &EvalResult{Pass: pass, FetchFromServer: server}
}

func getFromUser(user types.StatsigUser, field string) interface{} {
	switch strings.ToLower(field) {
	case "userid":
	case "user_id":
		return user.UserID
	case "email":
		return user.Email
	case "ip":
	case "ipaddress":
	case "ip_address":
		return user.IpAddress
	case "useragent":
	case "user_agent":
		return user.UserAgent
	case "country":
		return user.Country
	case "locale":
		return user.Locale
	case "clientversion":
	case "client_version":
		return user.ClientVersion
	default:
		// ok == true means field actually exists in user.Custom
		if val, ok := user.Custom[field]; ok {
			return val
		}
		if val, ok := user.Custom[strings.ToLower(field)]; ok {
			return val
		}
	}
	return nil
}

func getFromUserAgent(user types.StatsigUser, field string, parser *uaparser.Parser) string {
	client := parser.Parse(user.UserAgent)
	switch strings.ToLower(field) {
	case "os_name":
	case "osname":
		return client.Os.Family
	case "os_version":
	case "osversion":
		return strings.Join(removeEmptyStrings([]string{client.Os.Major, client.Os.Minor, client.Os.Patch, client.Os.PatchMinor}), ".")
	case "browser_name":
	case "browsername":
		return client.UserAgent.Family
	case "browser_version":
	case "browserversion":
		return strings.Join(removeEmptyStrings([]string{client.UserAgent.Major, client.UserAgent.Minor, client.UserAgent.Patch}), ".")
	}
	return ""
}

func getFromIP(user types.StatsigUser, field string, lookup *countrylookup.CountryLookup) string {
	if strings.ToLower(field) != "country" || user.IpAddress == "" {
		return ""
	}
	res, ok := lookup.LookupIpString(user.IpAddress)
	if ok {
		return res
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

func getHash(key string) uint64 {
	hasher := sha256.New()
	bytes := []byte(key)
	hasher.Write(bytes)
	return binary.BigEndian.Uint64(hasher.Sum(nil))
}

func getNumericValue(a interface{}) (float64, bool) {
	switch a.(type) {
	case int:
		return float64(a.(int)), true
	case int32:
		return float64(a.(int32)), true
	case int64:
		return float64(a.(int64)), true
	case float32:
		return float64(a.(float32)), true
	case float64:
		return a.(float64), true
	case string:
		s := string(a.(string))
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
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
	if reflect.TypeOf(s1).Kind() != reflect.String || reflect.TypeOf(s2).Kind() != reflect.String {
		return false
	}

	str1 := s1.(string)
	str2 := s2.(string)
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

func compare(a interface{}, b interface{}, ignoreCase bool) bool {
	return reflect.DeepEqual(a, b) ||
		compareNumbers(a, b, func(x, y float64) bool { return x == y }) ||
		compareStrings(a, b, ignoreCase, func(s1, s2 string) bool { return s1 == s2 })
}

func getTime(a interface{}) time.Time {
	var t_sec time.Time
	var t_msec time.Time
	switch a.(type) {
	case float64:
		t_sec = time.Unix(int64(a.(float64)), 0)
		t_msec = time.Unix(int64(a.(float64))/1000, 0)
	case int64:
		t_sec = time.Unix(a.(int64), 0)
		t_msec = time.Unix(a.(int64)/1000, 0)
	case int32:
		t_sec = time.Unix(int64(a.(int32)), 0)
		t_msec = time.Unix(int64(a.(int32))/1000, 0)
	case int:
		t_sec = time.Unix(int64(a.(int)), 0)
		t_msec = time.Unix(int64(a.(int))/1000, 0)
	case string:
		v, err := strconv.ParseInt(a.(string), 10, 64)
		if err != nil {
			t_sec = time.Unix(v, 0)
			t_msec = time.Unix(v/1000, 0)
		}
	}
	if t_sec.Year() > time.Now().Year()+100 {
		return t_msec
	}
	return t_sec
}
