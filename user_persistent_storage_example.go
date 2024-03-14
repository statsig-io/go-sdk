package statsig

import (
	"errors"
)

type userPersistentStorageExample struct {
	store      map[string]string
	loadCalled int
	saveCalled int
}

func (d *userPersistentStorageExample) Load(key string) (string, bool) {
	d.loadCalled++
	val, ok := d.store[key]
	return val, ok
}

func (d *userPersistentStorageExample) Save(key string, value string) {
	d.saveCalled++
	d.store[key] = value
}

type brokenUserPersistentStorageExample struct{}

func (d *brokenUserPersistentStorageExample) Load(string) (string, bool) {
	panic(errors.New("invalid Load function"))
}

func (d *brokenUserPersistentStorageExample) Save(string, string) {
	panic(errors.New("invalid Save function"))
}
