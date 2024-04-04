package statsig

import (
	"fmt"
)

type userPersistentStorageUtils struct {
	storage IUserPersistentStorage
}

func newUserPersistentStorageUtils(options *Options) *userPersistentStorageUtils {
	return &userPersistentStorageUtils{
		storage: options.UserPersistentStorage,
	}
}

func (p *userPersistentStorageUtils) load(user User, idType string) UserPersistedValues {
	if p.storage == nil {
		return nil
	}

	key := getStorageKey(user, idType)

	logError := func(err error) {
		Logger().LogError(fmt.Sprintf("Failed to load key (%s) from UserPersistentStorage (%s)\n", key, err.Error()))
	}

	defer func() {
		if err := recover(); err != nil {
			logError(toError(err))
		}
	}()

	storedValues, exists := p.storage.Load(key)
	if !exists {
		return nil
	}

	return storedValues
}

func (p *userPersistentStorageUtils) save(user User, idType string, configName string, evaluation *evalResult) {
	if p.storage == nil {
		return
	}

	key := getStorageKey(user, idType)

	logError := func(err error) {
		Logger().LogError(fmt.Sprintf("Failed to save key (%s) to UserPersistentStorage (%s)\n", key, err.Error()))
	}

	defer func() {
		if err := recover(); err != nil {
			logError(toError(err))
		}
	}()

	p.storage.Save(key, configName, evaluation.toStickyValues())
}

func (p *userPersistentStorageUtils) delete(user User, idType string, configName string) {
	if p.storage == nil {
		return
	}

	key := getStorageKey(user, idType)

	logError := func(err error) {
		Logger().LogError(fmt.Sprintf("Failed to save key (%s) to UserPersistentStorage (%s)\n", key, err.Error()))
	}

	defer func() {
		if err := recover(); err != nil {
			logError(toError(err))
		}
	}()

	p.storage.Delete(key, configName)
}

func getStorageKey(user User, idType string) string {
	var unitID string
	if idType == "userID" {
		unitID = user.UserID
	} else {
		unitID = user.CustomIDs[idType]
	}
	return fmt.Sprintf("%s:%s", unitID, idType)
}
