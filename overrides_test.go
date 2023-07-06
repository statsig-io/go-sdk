package statsig

import (
	"reflect"
	"testing"
)

func TestOverrides(t *testing.T) {
	InitializeGlobalOutputLogger(getStatsigTestLoggerOptions(t))
	c := NewClientWithOptions(secret, &Options{
		LocalMode: true,
	})

	user := User{
		UserID: "123",
		Email:  "123@gmail.com",
	}
	gateDefault := c.CheckGate(user, "any_gate")
	if gateDefault != false {
		t.Errorf("Failed to get default value for a gate when in LocalMode")
	}

	c.OverrideGate("any_gate", true)
	gateOverride := c.CheckGate(user, "any_gate")
	if gateOverride != true {
		t.Errorf("Failed to get override value for a gate when in LocalMode")
	}

	c.OverrideGate("any_gate", false)
	newGateOverride := c.CheckGate(user, "any_gate")
	if newGateOverride != false {
		t.Errorf("Failed to get override value for a gate when in LocalMode")
	}

	configDefault := c.GetConfig(user, "any_config")
	if len(configDefault.Value) != 0 {
		t.Errorf("Failed to get default value for a config when in LocalMode")
	}

	config := make(map[string]interface{})
	config["test"] = 123

	c.OverrideConfig("any_config", config)
	configOverride := c.GetConfig(user, "any_config")
	if !reflect.DeepEqual(configOverride.Value, config) {
		t.Errorf("Failed to get override value for a config when in LocalMode")
	}

	newConfig := make(map[string]interface{})
	newConfig["test"] = 456
	newConfig["test2"] = "hello"

	c.OverrideConfig("any_config", newConfig)
	newConfigOverride := c.GetConfig(user, "any_config")
	if !reflect.DeepEqual(newConfigOverride.Value, newConfig) {
		t.Errorf("Failed to get override value for a config when in LocalMode")
	}

	layer := make(map[string]interface{})
	layer["test"] = 123

	c.OverrideLayer("any_layer", layer)
	layerOverride := c.GetLayer(user, "any_layer")
	if !reflect.DeepEqual(layerOverride.Value, layer) {
		t.Errorf("Failed to get override value for a layer when in LocalMode")
	}

	newLayer := make(map[string]interface{})
	newLayer["test"] = 456
	newLayer["test2"] = "hello"

	c.OverrideLayer("any_layer", newLayer)
	newLayerOverride := c.GetLayer(user, "any_layer")
	if !reflect.DeepEqual(newLayerOverride.Value, newLayer) {
		t.Errorf("Failed to get override value for a layer when in LocalMode")
	}
}
