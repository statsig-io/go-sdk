package statsig

import (
	"fmt"
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

	set.mu.Lock()
	set.store = make(map[string]struct{})
	set.mu.Unlock()

	if set.Contains("key1") {
		t.Errorf("key1 should not exist after reset")
	}
	if set.Contains("key2") {
		t.Errorf("key2 should not exist after reset")
	}
}

func TestTTLSet_MaxSize(t *testing.T) {
	set := NewTTLSetWithMaxSize(minTTLSetMaxSize)
	for i := 0; i < minTTLSetMaxSize; i++ {
		set.Add(fmt.Sprintf("key_%d", i))
	}
	set.Add("overflow")

	if set.Contains("key_0") || set.Contains("key_1") {
		t.Errorf("existing keys should be cleared when max size is reached")
	}
	if !set.Contains("overflow") {
		t.Errorf("new key should still exist after clear-on-overflow insertion")
	}
}

func TestTTLSet_MinSize(t *testing.T) {
	set := NewTTLSetWithMaxSize(1)
	if set.maxSize != minTTLSetMaxSize {
		t.Errorf("max size should not be less than minimum")
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
