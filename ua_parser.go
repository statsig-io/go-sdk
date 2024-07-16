package statsig

import (
	"sync"

	"github.com/ua-parser/uap-go/uaparser"
)

type uaParser struct {
	parser  *uaparser.Parser
	wg      sync.WaitGroup
	options UAParserOptions
	mu      sync.RWMutex
}

func newUAParser(options UAParserOptions) *uaParser {
	uaParser := &uaParser{
		parser:  nil,
		wg:      sync.WaitGroup{},
		options: options,
	}
	return uaParser
}

func (u *uaParser) isReady() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.parser != nil
}

func (u *uaParser) init(lazyLoadOverride bool) {
	if u.options.Disabled {
		return
	}
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		u.mu.Lock()
		u.parser = uaparser.NewFromSaved()
		u.mu.Unlock()
	}()
	u.mu.Lock()
	lazyLoad := u.options.LazyLoad
	if lazyLoadOverride {
		lazyLoad = lazyLoadOverride
	}
	u.mu.Unlock()
	if !lazyLoad {
		u.wg.Wait()
	}
}

func (u *uaParser) parse(ua string) *uaparser.Client {
	if u.options.Disabled {
		return nil
	}
	if u.options.EnsureLoaded {
		u.wg.Wait()
	}
	if u.isReady() {
		return u.parser.Parse(ua)
	}
	return nil
}
