package statsig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type data struct {
	Entries []entry `json:"data"`
}

type entry struct {
	User    User                      `json:"user"`
	Gates   map[string]bool           `json:"feature_gates"`
	GatesV2 map[string]gateTestData   `json:"feature_gates_v2"`
	Configs map[string]configTestData `json:"dynamic_configs"`
	Layers  map[string]layerTestData  `json:"layer_configs"`
}

type gateTestData struct {
	Name               string              `json:"name"`
	Value              bool                `json:"value"`
	RuleID             string              `json:"rule_id"`
	SecondaryExposures []map[string]string `json:"secondary_exposures"`
}

type configTestData struct {
	Name               string                 `json:"name"`
	Value              map[string]interface{} `json:"value"`
	RuleID             string                 `json:"rule_id"`
	GroupName          string                 `json:"group_name"`
	SecondaryExposures []map[string]string    `json:"secondary_exposures"`
}

type layerTestData struct {
	Name                          string                 `json:"name"`
	Value                         map[string]interface{} `json:"value"`
	RuleID                        string                 `json:"rule_id"`
	GroupName                     string                 `json:"group_name"`
	SecondaryExposures            []map[string]string    `json:"secondary_exposures"`
	UndelegatedSecondaryExposures []map[string]string    `json:"undelegated_secondary_exposures"`
}

var secret string
var testAPIs = []string{
	"https://statsigapi.net/v1",
	"https://staging.statsigapi.net/v1",
}
var debugLogFile = "tmp/tests.log"

func getOutputLoggerOptionsForTest(t *testing.T) OutputLoggerOptions {
	return OutputLoggerOptions{
		LogCallback: func(message string, err error) {
			var mu sync.RWMutex
			mu.RLock()
			_ = os.MkdirAll(filepath.Dir(debugLogFile), 0770)
			f, e := os.OpenFile(debugLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			mu.RUnlock()
			if e != nil {
				fmt.Println(e.Error())
			}
			defer f.Close()
			mu.Lock()
			_, e = f.WriteString(fmt.Sprintf("(%s) %s", t.Name(), message))
			fmt.Fprint(os.Stderr, err)
			mu.Unlock()
			if e != nil {
				fmt.Println(e.Error())
			}
		},
		DisableInitDiagnostics: false,
		DisableSyncDiagnostics: true,
	}
}

func getStatsigLoggerOptionsForTest() StatsigLoggerOptions {
	return StatsigLoggerOptions{
		DisableInitDiagnostics: true,
		DisableSyncDiagnostics: true,
		DisableApiDiagnostics:  true,
	}
}

func TestMain(m *testing.M) {
	secret = os.Getenv("test_api_key")
	if secret == "" {
		absPath, _ := filepath.Abs("../ops/secrets/prod_keys/statsig-rulesets-eval-consistency-test-secret.key")
		bytes, err := os.ReadFile(absPath)
		if err != nil {
			panic("THIS TEST IS EXPECTED TO FAIL FOR NON-STATSIG EMPLOYEES! If this is the only test failing, please proceed to submit a pull request. If you are a Statsig employee, chat with jkw.")
		}
		secret = string(bytes)
	}
	os.Remove(debugLogFile)
	swallowStderr(func() {
		os.Exit(m.Run())
	})
	ShutdownAndDangerouslyClearInstance()
}

func TestEvaluation(t *testing.T) {
	for _, api := range testAPIs {
		testHelper(api, t)
	}
}

func testHelper(apiOverride string, t *testing.T) {
	t.Logf("Testing for " + apiOverride)
	InitializeGlobalOutputLogger(getOutputLoggerOptionsForTest(t))
	c := NewClientWithOptions(secret, &Options{API: apiOverride})
	var d data
	_, err := c.transport.post("/rulesets_e2e_test", nil, &d, RequestOptions{})

	if err != nil || len(d.Entries) == 0 {
		t.Errorf("Could not download test data")
	}

	gateChecks := len(d.Entries[0].GatesV2) * 3
	configChecks := len(d.Entries[0].Configs) * 3
	layerChecks := len(d.Entries[0].Layers) * 4

	var totalChecks = (gateChecks + configChecks + layerChecks) * len(d.Entries)
	var checks = 0
	for _, entry := range d.Entries {
		u := entry.User
		for gate, serverResult := range entry.GatesV2 {
			sdkResult := c.evaluator.checkGate(u, gate)
			if sdkResult.Pass != serverResult.Value {
				t.Errorf("Values are different for gate %s. SDK got %t but server is %t. User is %+v",
					gate, sdkResult.Pass, serverResult.Value, u)
			}

			if sdkResult.RuleID != serverResult.RuleID {
				t.Errorf("Rule IDs are different for gate %s. SDK got %s but server is %s. User is %+v",
					gate, sdkResult.RuleID, serverResult.RuleID, u)
			}

			if !compareSecondaryExp(sdkResult.SecondaryExposures, serverResult.SecondaryExposures) {
				t.Errorf("Secondary exposures are different for gate %s. SDK got %s but server is %s",
					gate, sdkResult.SecondaryExposures, serverResult.SecondaryExposures)
			}
			checks += 3
		}

		for config, serverResult := range entry.Configs {
			sdkResult := c.evaluator.getConfig(u, config, nil)
			if !reflect.DeepEqual(sdkResult.ConfigValue.Value, serverResult.Value) {
				t.Errorf("Values are different for config %s. SDK got %s but server is %s. User is %+v",
					config, sdkResult.ConfigValue.Value, serverResult.Value, u)
			}

			if sdkResult.RuleID != serverResult.RuleID {
				t.Errorf("Rule IDs are different for config %s. SDK got %s but server is %s",
					config, sdkResult.RuleID, serverResult.RuleID)
			}

			if sdkResult.ConfigValue.GroupName != serverResult.GroupName {
				t.Errorf("Group Names are different for config %s. SDK got %s but server is %s. User is %+v",
					config, sdkResult.ConfigValue.GroupName, serverResult.GroupName, u)
			}

			if !compareSecondaryExp(sdkResult.SecondaryExposures, serverResult.SecondaryExposures) {
				t.Errorf("Secondary exposures are different for config %s. SDK got %s but server is %s",
					config, sdkResult.SecondaryExposures, serverResult.SecondaryExposures)
			}
			checks += 3
		}

		for layer, serverResult := range entry.Layers {
			sdkResult := c.evaluator.getLayer(u, layer)
			if !reflect.DeepEqual(sdkResult.ConfigValue.Value, serverResult.Value) {
				t.Errorf("Values are different for layer %s. SDK got %s but server is %s. User is %+v",
					layer, sdkResult.ConfigValue.Value, serverResult.Value, u)
			}

			if sdkResult.RuleID != serverResult.RuleID {
				t.Errorf("Rule IDs are different for layer %s. SDK got %s but server is %s",
					layer, sdkResult.RuleID, serverResult.RuleID)
			}

			if sdkResult.ConfigValue.GroupName != serverResult.GroupName {
				t.Errorf("Group Names are different for layer %s. SDK got %s but server is %s. User is %+v",
					layer, sdkResult.ConfigValue.GroupName, serverResult.GroupName, u)
			}

			if !compareSecondaryExp(sdkResult.SecondaryExposures, serverResult.SecondaryExposures) {
				t.Errorf("Secondary exposures are different for layer %s. SDK got %s but server is %s",
					layer, sdkResult.SecondaryExposures, serverResult.SecondaryExposures)
			}

			if !compareSecondaryExp(sdkResult.UndelegatedSecondaryExposures, serverResult.UndelegatedSecondaryExposures) {
				t.Errorf("Undelegated Secondary exposures are different for layer %s. SDK got %s but server is %s",
					layer, sdkResult.UndelegatedSecondaryExposures, serverResult.UndelegatedSecondaryExposures)
			}
			checks += 4
		}
	}
	if totalChecks != checks {
		t.Errorf("Expected to perform %d but only checked %d times for %s.", totalChecks, checks, apiOverride)
	}
}

func TestStatsigLocalMode(t *testing.T) {
	user := &User{UserID: "test"}
	local := &Options{
		LocalMode: true,
	}
	InitializeGlobalOutputLogger(getOutputLoggerOptionsForTest(t))
	localC := NewClientWithOptions("", local)
	network := &Options{}
	netC := NewClientWithOptions(secret, network)
	localGate := localC.CheckGate(*user, "test_public")
	if localGate {
		t.Errorf("Local mode should always return false for gate checks")
	}
	netGate := netC.CheckGate(*user, "test_public")
	if !netGate {
		t.Errorf("Network mode should work")
	}

	localConfig := localC.GetConfig(*user, "test_custom_config")
	netConfig := netC.GetConfig(*user, "test_custom_config")
	if len(localConfig.Value) != 0 {
		t.Errorf("Local mode should always return false for gate checks")
	}
	if len(netConfig.Value) == 0 {
		t.Errorf("Network mode should fetch configs")
	}
}

func compareSecondaryExp(v1 []map[string]string, v2 []map[string]string) bool {
	if (v1 == nil && v2 != nil) || (v2 == nil && v1 != nil) {
		return false
	}
	if v1 == nil {
		v1 = []map[string]string{}
	}
	if v2 == nil {
		v2 = []map[string]string{}
	}
	return reflect.DeepEqual(v1, v2)
}
