package statsig

import (
	"statsig/pkg/types"
	"testing"
)

func TestInitialize(t *testing.T) {
	Initialize("secret-9IWfdzNwExEYHEW4YfOQcFZ4xreZyFkbOXHaNbPsMwW")
	user := types.StatsigUser{
		UserID: "jkw",
	}
	gate := CheckGate(user, "test_public")
	if !gate {
		t.Errorf("Public 100 gate returned false")
	}

	gate = CheckGate(user, "test_ua")
	if gate {
		t.Errorf("UA get returned true")
	}

	// config := GetConfig(user, "operating_system_config")
	// if config.Name != "operating_system_config" {
	// 	t.Errorf("Wrong dynamic config")
	// }
	// if config.RuleID != "default" {
	// 	t.Errorf("Wrong dynamic config rule")
	// }

	//Test event logging
	event := &types.StatsigEvent{
		User:  user,
		Value: "hi there",
		Metadata: map[string]string{
			"sdk language": "go",
		},
	}

	LogEvent(*event)
	// test polling
	// time.Sleep(12 * time.Second)
	// LogEvent(*event)
	// time.Sleep(12 * time.Second)
	// Shutdown()
	// LogEvent(*event)
	// time.Sleep(12 * time.Second)
	// LogEvent(*event)
	// time.Sleep(12 * time.Second)
	// LogEvent(*event)

}
