package statsig

import (
	"testing"
	"time"
)

func TestTTLSet_AddAndContains(t *testing.T) {
	set := NewTTLSet()
	set.Add("key1")
	if !set.Contains("key1") {
		t.Errorf("key1 should exist in the set")
	}
	if set.Contains("key2") {
		t.Errorf("key2 should not exist in the set")
	}
}

func TestTTLSet_Reset(t *testing.T) {
	set := NewTTLSet()
	set.Add("key1")
	set.Add("key2")
	set.store = make(map[string]struct{})
	if set.Contains("key1") {
		t.Errorf("key1 should not exist after reset")
	}
	if set.Contains("key2") {
		t.Errorf("key2 should not exist after reset")
	}
}

func TestTTLSet_StartResetThread(t *testing.T) {
	set := NewTTLSet()
	set.resetInterval = 10 * time.Millisecond
	set.StartResetThread()

	set.Add("key1")
	time.Sleep(20 * time.Millisecond)
	if set.Contains("key1") {
		t.Errorf("key1 should not exist after automatic reset")
	}

	set.Shutdown()
}

func TestTTLSet_Shutdown(t *testing.T) {
	set := NewTTLSet()
	set.resetInterval = 10 * time.Millisecond
	set.StartResetThread()

	set.Add("key1")
	set.Shutdown()
	time.Sleep(20 * time.Millisecond)

	set.Add("key2")
	if !set.Contains("key2") {
		t.Errorf("shutdown should prevent automatic reset")
	}
}
