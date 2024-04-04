package statsig

import (
	"errors"
)

type userPersistentStorageExample struct {
	store        map[string]UserPersistedValues
	loadCalled   int
	saveCalled   int
	deleteCalled int
}

func (d *userPersistentStorageExample) Load(key string) (UserPersistedValues, bool) {
	d.loadCalled++
	val, ok := d.store[key]
	return val, ok
}

func (d *userPersistentStorageExample) Save(key string, configName string, value StickyValues) {
	d.saveCalled++
	if _, ok := d.store[key]; !ok {
		d.store[key] = make(UserPersistedValues)
	}
	d.store[key][configName] = value
}

func (d *userPersistentStorageExample) Delete(key string, configName string) {
	d.deleteCalled++
	delete(d.store[key], configName)
}

type brokenUserPersistentStorageExample struct{}

func (d *brokenUserPersistentStorageExample) Load(key string) (UserPersistedValues, bool) {
	panic(errors.New("invalid Load function"))
}

func (d *brokenUserPersistentStorageExample) Save(key string, configName string, value StickyValues) {
	panic(errors.New("invalid Save function"))
}

func (d *brokenUserPersistentStorageExample) Delete(key string, configName string) {
	panic(errors.New("invalid Delete function"))
}
