package statsig

import "testing"

func TestOverrides(t *testing.T) {
	c := NewClientWithOptions(secret, &Options{LocalMode: true})

	user := User{
		UserID: "123",
		Email:  "123@gmail.com",
	}
	gateDefault := c.CheckGate(user, "any_gate")
	if gateDefault != false {
		t.Errorf("Failed to get default value for a gate when in LocalMode")
	}

	c.OverrideGate(user, "any_gate", true)
	gateOverride := c.CheckGate(user, "any_gate")
	if gateOverride != true {
		t.Errorf("Failed to get override value for a gate when in LocalMode")
	}

	configDefault := c.GetConfig(user, "any_config")
	if len(configDefault.Value) != 0 {
		t.Errorf("Failed to get default value for a config when in LocalMode")
	}

	config := make(map[string]interface{})
	config["test"] = 123

	c.OverrideConfig(user, "any_config", config)
	configOverride := c.GetConfig(user, "any_config")
	if configOverride.Value["test"] != 123 {
		t.Errorf("Failed to get override value for a config when in LocalMode")
	}
}
