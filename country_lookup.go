package statsig

import (
	"sync"

	countrylookup "github.com/statsig-io/ip3country-go"
)

type countryLookup struct {
	lookup  *countrylookup.CountryLookup
	wg      sync.WaitGroup
	options IPCountryOptions
	mu      sync.RWMutex
}

func newCountryLookup(options IPCountryOptions) *countryLookup {
	countryLookup := &countryLookup{
		lookup:  nil,
		wg:      sync.WaitGroup{},
		options: options,
	}
	countryLookup.delayedSetup()
	return countryLookup
}

func (c *countryLookup) isReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lookup != nil
}

func (c *countryLookup) delayedSetup() {
	if c.options.Disabled {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.mu.Lock()
		c.lookup = countrylookup.New()
		c.mu.Unlock()
	}()
}

func (c *countryLookup) init() {
	if !c.options.LazyLoad {
		c.ensureLoaded()
	}
}

func (c *countryLookup) ensureLoaded() {
	if c.options.Disabled {
		return
	}
	c.wg.Wait()
}

func (c *countryLookup) lookupIp(ip string) (string, bool) {
	if c.options.Disabled {
		return "", false
	}
	if c.options.EnsureLoaded {
		c.wg.Wait()
	}
	if c.isReady() {
		val, ok := c.lookup.LookupIp(ip)
		return val, ok
	}
	return "", false
}
