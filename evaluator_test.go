package statsig

import "testing"

func TestStringComparsigon(t *testing.T) {
	eq := func(s1, s2 string) bool { return s1 == s2 }

	if !compareStrings("a", "a", true, eq) {
		t.Error("Expected string equality check to pass")
	}
	if !compareStrings("a", "A", true, eq) {
		t.Error("Expected case-insensitive string equality check to pass")
	}
	if !compareStrings(true, "true", true, eq) {
		t.Error("Expected boolean to string equality check to pass")
	}
	var numInt = 1
	if !compareStrings(numInt, "1", true, eq) {
		t.Error("Expected integer to string equality check to pass")
	}

	type StringDefinition string
	const (
		A1 StringDefinition = "a"
	)
	if !compareStrings(A1, "a", true, eq) {
		t.Error("Expected string custom definition equality check to pass")
	}

	type StringAlias = string
	const (
		A2 StringAlias = "a"
	)
	if !compareStrings(A2, "a", true, eq) {
		t.Error("Expected string alias equality check to pass")
	}
}

func TestNumericComparsigon(t *testing.T) {
	eq := func(x, y float64) bool { return x == y }

	var numInt = 1
	if !compareNumbers(numInt, 1, eq) {
		t.Error("Expected int equality check to pass")
	}
	var numUInt uint = 1
	if !compareNumbers(numUInt, 1, eq) {
		t.Error("Expected uint equality check to pass")
	}
	var numFloat32 float32 = 1
	if !compareNumbers(numFloat32, 1, eq) {
		t.Error("Expected float32 equality check to pass")
	}
	var numFloat64 float64 = 1
	if !compareNumbers(numFloat64, 1, eq) {
		t.Error("Expected float64 equality check to pass")
	}

	type IntDefinition int
	const (
		Int1 IntDefinition = 1
	)
	if !compareNumbers(Int1, 1, eq) {
		t.Error("Expected int custom definition equality check to pass")
	}

	type IntAlias = int
	const (
		Int2 IntAlias = 1
	)
	if !compareNumbers(Int2, 1, eq) {
		t.Error("Expected int alias equality check to pass")
	}
}
