package statsig

import (
	"fmt"
	"os"
	"testing"
)

func TestUserPersistentStorage(t *testing.T) {
	persistentStorage := &userPersistentStorageExample{store: make(map[string]UserPersistedValues)}
	bytes, _ := os.ReadFile("download_config_specs_sticky_experiments.json")
	opts := &Options{
		OutputLoggerOptions:   getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions:  getStatsigLoggerOptionsForTest(t),
		BootstrapValues:       string(bytes),
		UserPersistentStorage: persistentStorage,
	}
	InitializeWithOptions("secret-key", opts)
	userInControl := User{UserID: "vj"}
	userInTest := User{UserID: "hunter2"}
	userNotInExp := User{UserID: "gb"}
	expName := "the_allocated_experiment"

	// Control group
	exp := GetExperiment(userInControl, expName)
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	// Test group
	exp = GetExperiment(userInTest, expName)
	if exp.GroupName != "Test" {
		t.Errorf("Expected: Test. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	// Not allocated to the experiment
	exp = GetExperiment(userNotInExp, expName)
	if exp.RuleID != "layerAssignment" {
		t.Errorf("Expected: layerAssignment. Received: %s", exp.RuleID)
	}

	// At this point, we have not opted in to sticky
	if len(persistentStorage.store) != 0 {
		t.Errorf("Expected persistent storage to be empty")
	}
	if persistentStorage.saveCalled != 0 {
		t.Errorf("Expected persistent storage Save to have not been called")
	}

	// Control group with persistent storage enabled
	// (should save to storage, but evaluate as normal until next call)
	persistedValues := GetUserPersistedValues(userInControl, "userID")
	exp = GetExperimentWithOptions(userInControl, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	// Test group with persistent storage enabled
	// (should save to storage, but evaluate as normal until next call)
	persistedValues = GetUserPersistedValues(userInTest, "userID")
	exp = GetExperimentWithOptions(userInTest, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Test" {
		t.Errorf("Expected: Test. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	// Verify that persistent storage has been updated
	if len(persistentStorage.store) != 2 {
		t.Errorf("Expected persistent storage to have size 2, Received: %d", len(persistentStorage.store))
	}
	if persistentStorage.saveCalled != 2 {
		t.Errorf("Expected persistent storage Save to have been called 2 times, Received: %d", persistentStorage.saveCalled)
	}

	// Use sticky bucketing with valid persisted values
	// (Should override userInControl to the first evaluation of userInControl)
	persistedValues = GetUserPersistedValues(userInControl, "userID")
	exp = GetExperimentWithOptions(userInControl, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonPersisted {
		t.Errorf("Expected: %s. Received: %s", reasonPersisted, exp.EvaluationDetails.reason)
	}
	persistedValues = GetUserPersistedValues(userInTest, "userID")
	exp = GetExperimentWithOptions(userInTest, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Test" {
		t.Errorf("Expected: Test. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonPersisted {
		t.Errorf("Expected: %s. Received: %s", reasonPersisted, exp.EvaluationDetails.reason)
	}

	// Use sticky bucketing with valid persisted values to assign a user that would otherwise be unallocated
	// (Should override userNotInExp to the first evaluation of userInControl)
	persistedValues = GetUserPersistedValues(userInControl, "userID")
	exp = GetExperimentWithOptions(userNotInExp, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonPersisted {
		t.Errorf("Expected: %s. Received: %s", reasonPersisted, exp.EvaluationDetails.reason)
	}

	// Use sticky bucketing with valid persisted values for an unallocated user
	// (Should not override since there are no persisted values)
	persistedValues = GetUserPersistedValues(userNotInExp, "userID")
	exp = GetExperimentWithOptions(userNotInExp, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.RuleID != "layerAssignment" {
		t.Errorf("Expected: layerAssignment. Received: %s", exp.RuleID)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	// Use sticky bucketing on a different ID type that hasn't been saved to storage
	// (Should not override since there are no persisted values)
	persistedValues = GetUserPersistedValues(userInTest, "stableID")
	exp = GetExperimentWithOptions(userInTest, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Test" {
		t.Errorf("Expected: Test. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	// Verify that persistent storage has been updated
	if len(persistentStorage.store) != 2 {
		t.Errorf("Expected persistent storage to have size 2, Received: %d", len(persistentStorage.store))
	}
	if persistentStorage.saveCalled != 3 {
		t.Errorf("Expected persistent storage Save to have been called 3 times, Received: %d", persistentStorage.saveCalled)
	}

	// Verify that persisted values are deleted once the experiment is no longer active
	bytes, _ = os.ReadFile("download_config_specs_sticky_experiments_inactive.json")
	opts.BootstrapValues = string(bytes)
	ShutdownAndDangerouslyClearInstance()
	InitializeWithOptions("secret-key", opts)
	stillActiveExpName := "another_allocated_experiment_still_active"

	persistedValues = GetUserPersistedValues(userInControl, "userID")
	exp = GetExperimentWithOptions(userInControl, stillActiveExpName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	userInControlPersistedValues := persistentStorage.store[fmt.Sprintf("%s:userID", userInControl.UserID)]
	if _, ok := userInControlPersistedValues[stillActiveExpName]; !ok {
		t.Errorf("Expected %s to exist in user persisted storage for user %s", stillActiveExpName, userInControl.UserID)
	}

	persistedValues = GetUserPersistedValues(userInControl, "userID")
	exp = GetExperimentWithOptions(userInControl, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	userInControlPersistedValues = persistentStorage.store[fmt.Sprintf("%s:userID", userInControl.UserID)]
	if _, ok := userInControlPersistedValues[expName]; ok {
		t.Errorf("Expected %s to not exist in user persisted storage for user %s", expName, userInControl.UserID)
	}

	// Verify that persisted values are deleted once an experiment is evaluated without persisted values (opted-out)
	exp = GetExperiment(userInTest, expName)
	if exp.GroupName != "Test" {
		t.Errorf("Expected: Test. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	userInTestPersistedValues := persistentStorage.store[fmt.Sprintf("%s:userID", userInTest.UserID)]
	if _, ok := userInTestPersistedValues[expName]; ok {
		t.Errorf("Expected %s to not exist in user persisted storage for user %s", expName, userInTest.UserID)
	}
}

func TestInvalidUserPersistentStorage(t *testing.T) {
	persistentStorage := &brokenUserPersistentStorageExample{}
	bytes, _ := os.ReadFile("download_config_specs_sticky_experiments.json")
	opts := &Options{
		OutputLoggerOptions:   getOutputLoggerOptionsForTest(t),
		StatsigLoggerOptions:  getStatsigLoggerOptionsForTest(t),
		BootstrapValues:       string(bytes),
		UserPersistentStorage: persistentStorage,
	}
	InitializeWithOptions("secret-key", opts)
	userInControl := User{UserID: "vj"}
	expName := "the_allocated_experiment"

	persistedValues := GetUserPersistedValues(userInControl, "userID")
	exp := GetExperimentWithOptions(userInControl, expName, &GetExperimentOptions{PersistedValues: persistedValues})
	if exp.GroupName != "Control" {
		t.Errorf("Expected: Control. Received: %s", exp.GroupName)
	}
	if exp.EvaluationDetails.reason != reasonBootstrap {
		t.Errorf("Expected: %s. Received: %s", reasonBootstrap, exp.EvaluationDetails.reason)
	}

	if err := recover(); err != nil {
		t.Errorf("Expected not to panic")
	}
}
