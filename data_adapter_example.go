package statsig

import (
	"errors"
	"sync"
)

type dataAdapterExample struct {
	store map[string]string
}

func (d dataAdapterExample) get(key string) string {
	return d.store[key]
}

func (d dataAdapterExample) set(key string, value string) {
	d.store[key] = value
}

func (d dataAdapterExample) initialize() {}

func (d dataAdapterExample) shutdown() {}

func (d dataAdapterExample) shouldBeUsedForQueryingUpdates(key string) bool {
	return false
}

type brokenDataAdapterExample struct{}

func (d brokenDataAdapterExample) get(key string) string {
	panic(errors.New("invalid get function"))
}

func (d brokenDataAdapterExample) set(key string, value string) {
	panic(errors.New("invalid set function"))
}

func (d brokenDataAdapterExample) initialize() {}

func (d brokenDataAdapterExample) shutdown() {}

func (d brokenDataAdapterExample) shouldBeUsedForQueryingUpdates(key string) bool {
	return false
}

type dataAdapterWithPollingExample struct {
	store map[string]string
	mu    sync.RWMutex
}

func (d *dataAdapterWithPollingExample) get(key string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.store[key]
}

func (d *dataAdapterWithPollingExample) set(key string, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store[key] = value
}

func (d *dataAdapterWithPollingExample) initialize() {}

func (d *dataAdapterWithPollingExample) shutdown() {}

func (d *dataAdapterWithPollingExample) shouldBeUsedForQueryingUpdates(key string) bool {
	return true
}
func (d *dataAdapterWithPollingExample) clearStore(key string) {
	d.set(key, "{\"feature_gates\":[],\"dynamic_configs\":[],\"layer_configs\":[],\"layers\":{},\"id_lists\":{},\"has_updates\":true,\"time\":1}")
}
