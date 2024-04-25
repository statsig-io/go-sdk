package statsig

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestBenchmarkLocalMode(t *testing.T) {
	secret = os.Getenv("test_api_key")
	defaultDuration := measureDuration(func() {
		options := &Options{LocalMode: false}
		NewClientWithOptions(secret, options)
	})
	localModeDuration := measureDuration(func() {
		options := &Options{LocalMode: true}
		NewClientWithOptions(secret, options)
	})

	fmt.Printf("Default duration: %s\n", defaultDuration)
	fmt.Printf("Local mode duration: %s\n", localModeDuration)

	if defaultDuration < localModeDuration {
		t.Error("Expected faster initialization with local mode")
	}
}

func TestBenchmarkUAParser(t *testing.T) {
	secret = os.Getenv("test_api_key")
	defaultDuration := measureDuration(func() {
		options := &Options{LocalMode: true}
		NewClientWithOptions(secret, options)
	})
	disabledDuration := measureDuration(func() {
		options := &Options{LocalMode: true, UAParserOptions: UAParserOptions{Disabled: true}}
		NewClientWithOptions(secret, options)
	})

	fmt.Printf("UA Parser default duration: %s\n", defaultDuration)
	fmt.Printf("UA Parser disabled duration: %s\n", disabledDuration)

	if defaultDuration < disabledDuration {
		t.Error("Expected faster initialization with disabled UA parsser")
	}
}

func measureDuration(f func()) time.Duration {
	start := time.Now()
	f()
	return time.Since(start)
}
