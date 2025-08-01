package statsig

import (
	"encoding/json"
	"reflect"
	"testing"
)

func doValidation(t *testing.T, c *configBase) {
	if c.Name != "test" {
		t.Errorf("Failed to set name")
	}
	if c.RuleID != "rule_id" {
		t.Errorf("Failed to set rule_id")
	}
	if c.GroupName != "group_name" {
		t.Errorf("Failed to set group_name")
	}
	if c.IDType != "id_type" {
		t.Errorf("Failed to set id_type")
	}

	if c.GetString("String", "abc") != "str" {
		t.Errorf("Failed to get string")
	}
	if c.GetString("Number", "abc") != "abc" {
		t.Errorf("Failed to use fallback string")
	}
	if c.GetString("Object", "def") != "def" {
		t.Errorf("Failed to use fallback string")
	}

	if c.GetNumber("String", 0.07) != 0.07 {
		t.Errorf("Failed to use fallback number")
	}
	if c.GetNumber("Number", 0.07) != 143.7 {
		t.Errorf("Failed to get number")
	}
	if c.GetNumber("Object", 4) != 4 {
		t.Errorf("Failed to use fallback number")
	}

	if !c.GetBool("String", true) {
		t.Errorf("Failed to use fallback boolean")
	}
	if !c.GetBool("Boolean", false) {
		t.Errorf("Failed to get boolean")
	}
	if c.GetBool("Object", false) {
		t.Errorf("Failed to use fallback boolean")
	}
}

func doValidationLayer(t *testing.T, c *Layer) {
	if c.Name != "test" {
		t.Errorf("Failed to set name")
	}
	if c.RuleID != "rule_id" {
		t.Errorf("Failed to set rule_id")
	}
	if c.GroupName != "group_name" {
		t.Errorf("Failed to set group_name")
	}
	if c.IDType != "id_type" {
		t.Errorf("Failed to set id type")
	}
	if c.GetString("String", "abc") != "str" {
		t.Errorf("Failed to get string")
	}
	if c.GetString("Number", "abc") != "abc" {
		t.Errorf("Failed to use fallback string")
	}
	if c.GetString("Object", "def") != "def" {
		t.Errorf("Failed to use fallback string")
	}

	if c.GetNumber("String", 0.07) != 0.07 {
		t.Errorf("Failed to use fallback number")
	}
	if c.GetNumber("Number", 0.07) != 143.7 {
		t.Errorf("Failed to get number")
	}
	if c.GetNumber("Object", 4) != 4 {
		t.Errorf("Failed to use fallback number")
	}

	if !c.GetBool("String", true) {
		t.Errorf("Failed to use fallback boolean")
	}
	if !c.GetBool("Boolean", false) {
		t.Errorf("Failed to get boolean")
	}
	if c.GetBool("Object", false) {
		t.Errorf("Failed to use fallback boolean")
	}
}

func TestBasic(t *testing.T) {
	jsonMap := make(map[string]interface{})
	_ = json.Unmarshal(
		[]byte(
			`{
				"Boolean": true,
				"Number": 143.7,
				"String": "str",
				"Object": {
					"NestedBool": false,
					"NestedNum": 37
				},
				"Array":[1,2,3]
			}`,
		),
		&jsonMap,
	)

	c := NewConfig(
		"test",
		jsonMap,
		"rule_id",
		"id_type",
		"group_name",
		nil,
	)
	doValidation(t, &c.configBase)

	l := NewLayer("test", jsonMap, "rule_id", "id_type", "group_name", nil, nil, "allocated_experiment_name")
	doValidationLayer(t, l)

	fallbackValues := make([]interface{}, 0)
	fallbackValues = append(fallbackValues, 4, 5, 6)
	if !reflect.DeepEqual(c.GetSlice("String", fallbackValues), fallbackValues) {
		t.Errorf("Failed to use fallback slice")
	}
	actualValues := make([]interface{}, 0)
	actualValues = append(actualValues, 1.0, 2.0, 3.0)
	if !reflect.DeepEqual(c.GetSlice("Array", fallbackValues), actualValues) {
		t.Errorf("Failed to get number array")
	}
}
