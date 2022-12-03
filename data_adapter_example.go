package statsig

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
