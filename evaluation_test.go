package statsig

import (
	"os"
	"path/filepath"
	"testing"
)

type data struct {
	Entries []entry `json:"data"`
}

type entry struct {
	User    User                     `json:"user"`
	Gates   map[string]bool          `json:"feature_gates"`
	Configs map[string]DynamicConfig `json:"dynamic_configs"`
}

var secret string
var testAPIs = []string{
	"https://api.statsig.com/v1",
	"https://us-west-2.api.statsig.com/v1",
	"https://us-east-2.api.statsig.com/v1",
	"https://ap-south-1.api.statsig.com/v1",
	"https://latest.api.statsig.com/v1",
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
	if err != nil {
		t.Errorf("Could not download test data")
	}

	for _, entry := range d.Entries {
		u := entry.User
		for gate, value := range entry.Gates {
			sdkV := c.CheckGate(u, gate)
			if sdkV != value {
				t.Errorf("%s failed for user %s: expected %t, got %t", gate, u, value, sdkV)
			}
		}
	}
}
