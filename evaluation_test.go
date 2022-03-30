package statsig

import (
	"os"
	"path/filepath"
	"reflect"
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
	SecondaryExposures []map[string]string    `json:"secondary_exposures"`
}

var secret string
var testAPIs = []string{
	"https://statsigapi.net/v1",
	"https://statsigapi.net/v1",
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
	os.Exit(m.Run())
}

func Test(t *testing.T) {
	for _, api := range testAPIs {
		test_helper(api, t)
	}
}

func test_helper(apiOverride string, t *testing.T) {
	t.Logf("Testing for " + apiOverride)
	c := NewClientWithOptions(secret, &Options{API: apiOverride})
	var d data
	err := c.transport.postRequest("/rulesets_e2e_test", nil, &d)

	if err != nil || len(d.Entries) == 0 {
		t.Errorf("Could not download test data")
	}

	var totalChecks = 3 * (len(d.Entries[0].GatesV2) + len(d.Entries[0].Configs)) * len(d.Entries)
	var checks = 0
	for _, entry := range d.Entries {
		u := entry.User
		for gate, serverResult := range entry.GatesV2 {
			sdkResult := c.evaluator.checkGate(u, gate)
			if sdkResult.Pass != serverResult.Value {
				t.Errorf("Values are different for gate %s. SDK got %t but server is %t. User is %s",
					gate, sdkResult.Pass, serverResult.Value, u)
			}

			if sdkResult.Id != serverResult.RuleID {
				t.Errorf("Rule IDs are different for gate %s. SDK got %s but server is %s",
					gate, sdkResult.Id, serverResult.RuleID)
			}

			if !compare_exposures(sdkResult.SecondaryExposures, serverResult.SecondaryExposures) {
				t.Errorf("Secondary exposures are different for gate %s. SDK got %s but server is %s",
					gate, sdkResult.SecondaryExposures, serverResult.SecondaryExposures)
			}
			checks += 3
		}

		for config, serverResult := range entry.Configs {
			sdkResult := c.evaluator.getConfig(u, config)
			if !reflect.DeepEqual(sdkResult.ConfigValue.Value, serverResult.Value) {
				t.Errorf("Values are different for config %s. SDK got %s but server is %s. User is %s",
					config, sdkResult.ConfigValue.Value, serverResult.Value, u)
			}

			if sdkResult.Id != serverResult.RuleID {
				t.Errorf("Rule IDs are different for config %s. SDK got %s but server is %s",
					config, sdkResult.Id, serverResult.RuleID)
			}

			if !compare_exposures(sdkResult.SecondaryExposures, serverResult.SecondaryExposures) {
				t.Errorf("Secondary exposures are different for config %s. SDK got %s but server is %s",
					config, sdkResult.SecondaryExposures, serverResult.SecondaryExposures)
			}
			checks += 3
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
	local_c := NewClientWithOptions("", local)
	network := &Options{}
	net_c := NewClientWithOptions(secret, network)
	local_gate := local_c.CheckGate(*user, "test_public")
	if local_gate {
		t.Errorf("Local mode should always return false for gate checks")
	}
	net_gate := net_c.CheckGate(*user, "test_public")
	if !net_gate {
		t.Errorf("Network mode should work")
	}

	local_config := local_c.GetConfig(*user, "test_custom_config")
	net_config := net_c.GetConfig(*user, "test_custom_config")
	if len(local_config.Value) != 0 {
		t.Errorf("Local mode should always return false for gate checks")
	}
	if len(net_config.Value) == 0 {
		t.Errorf("Network mode should fetch configs")
	}
}

func compare_exposures(v1 []map[string]string, v2 []map[string]string) bool {
	if v1 == nil {
		v1 = []map[string]string{}
	}
	if v2 == nil {
		v2 = []map[string]string{}
	}
	return reflect.DeepEqual(v1, v2)
}
