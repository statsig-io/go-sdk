package statsig

import "errors"

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

type brokenDataAdapterExample struct{}

func (d brokenDataAdapterExample) get(key string) string {
	panic(errors.New("invalid get function"))
}

func (d brokenDataAdapterExample) set(key string, value string) {
	panic(errors.New("invalid set function"))
}

func (d brokenDataAdapterExample) initialize() {}

func (d brokenDataAdapterExample) shutdown() {}
