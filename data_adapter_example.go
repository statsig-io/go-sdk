package statsig

import (
	"errors"
	"sync"
)

type dataAdapterExample struct {
	store map[string]string
	mu    sync.RWMutex
}

func (d *dataAdapterExample) Get(key string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.store[key]
}

func (d *dataAdapterExample) Set(key string, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[key] = value
}

func (d *dataAdapterExample) Initialize() {}

func (d *dataAdapterExample) Shutdown() {}

func (d *dataAdapterExample) ShouldBeUsedForQueryingUpdates(string) bool {
	return false
}

type brokenDataAdapterExample struct{}

func (d brokenDataAdapterExample) Get(string) string {
	panic(errors.New("invalid get function"))
}

func (d brokenDataAdapterExample) Set(string, string) {
	panic(errors.New("invalid set function"))
}

func (d brokenDataAdapterExample) Initialize() {}

func (d brokenDataAdapterExample) Shutdown() {}

func (d brokenDataAdapterExample) ShouldBeUsedForQueryingUpdates(string) bool {
	return false
}

type dataAdapterWithPollingExample struct {
	store map[string]string
	mu    sync.RWMutex
}

func (d *dataAdapterWithPollingExample) Get(key string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.store[key]
}

func (d *dataAdapterWithPollingExample) Set(key string, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[key] = value
}

func (d *dataAdapterWithPollingExample) Initialize() {}

func (d *dataAdapterWithPollingExample) Shutdown() {}

func (d *dataAdapterWithPollingExample) ShouldBeUsedForQueryingUpdates(string) bool {
	return true
}
func (d *dataAdapterWithPollingExample) clearStore(key string) {
	d.Set(key, "{\"feature_gates\":[],\"dynamic_configs\":[],\"layer_configs\":[],\"layers\":{},\"id_lists\":{},\"has_updates\":true,\"time\":1}")
}
