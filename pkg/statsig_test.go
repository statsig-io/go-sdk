package statsig

import (
    "testing"
	"statsig/pkg/types"
)

func TestInitialize(t *testing.T) {
	Initialize("secret-9IWfdzNwExEYHEW4YfOQcFZ4xreZyFkbOXHaNbPsMwW")
	user := types.StatsigUser{
		UserID: "jkw",
	}
	gate := CheckGate(user, "test_public")
	if (!gate) {
		t.Errorf("Public 100 gate returned false")
	}

	gate = CheckGate(user, "test_ua")
	if (gate) {
		t.Errorf("UA get returned true")
	}

	config := GetConfig(user, "operating_system_config")
	if (config.Name != "operating_system_config") {
		t.Errorf("Wrong dynamic config")
	}
	if (config.RuleID != "default") {
		t.Errorf("Wrong dynamic config rule")
	}

	// Test event logging
	// event := &types.StatsigEvent{
	// 	User: user,
	// 	EventName: "hi",
	// 	Value: 43,
	// 	Metadata: map[string]string{
	// 		"sdk language": "go",
	// 	},
	// }

	// for i := 0; i < 12; i++ {
	// 	LogEvent(*event)
	// }
}