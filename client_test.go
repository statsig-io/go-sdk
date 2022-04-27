package statsig

import (
	"sync"
	"testing"
	"time"
)

func TestNormalizeUserDataRace(t *testing.T) {
	const (
		goroutines = 10
		duration   = time.Second
	)
	options := Options{
		Environment: Environment{
			Params: map[string]string{
				"foo": "bar",
			},
			Tier: "awesome",
		},
	}
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for time.Since(start) < duration {
				normalizeUser(User{UserID: "cruise-llc"}, options)
			}
		}()
	}
	wg.Wait()
}
