package statsig

import (
	"os"
	"testing"
)

func TestCountryLookup(t *testing.T) {
	secret = os.Getenv("test_api_key")
	client := NewClient(secret)
	userPass := User{UserID: "123", IpAddress: "24.18.183.148"}  // Seattle, WA
	userFail := User{UserID: "123", IpAddress: "115.240.90.163"} // Mumbai, India (IN)
	if !client.CheckGate(userPass, "test_country") {
		t.Error("Expected user to pass test_country")
	}
	if client.CheckGate(userFail, "test_country") {
		t.Error("Expected user to fail test_country")
	}
}

func TestCountryLookupDisabled(t *testing.T) {
	options := &Options{IPCountryOptions: IPCountryOptions{Disabled: true}}
	secret = os.Getenv("test_api_key")
	client := NewClientWithOptions(secret, options)
	userPass := User{UserID: "123", IpAddress: "24.18.183.148"} // Seattle, WA
	if client.CheckGate(userPass, "test_country") {
		t.Error("Expected passing user to fail test_country if country lookup is disabled")
	}
}

func TestCountryLookupLazyLoad(t *testing.T) {
	options := &Options{IPCountryOptions: IPCountryOptions{LazyLoad: true}}
	secret = os.Getenv("test_api_key")
	client := NewClientWithOptions(secret, options)
	userPass := User{UserID: "123", IpAddress: "24.18.183.148"} // Seattle, WA
	waitForCondition(t, func() bool {
		return client.CheckGate(userPass, "test_country")
	})
}
