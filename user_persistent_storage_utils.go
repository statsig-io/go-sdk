package statsig

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type UserPersistedValues = map[string]map[string]interface{}

type userPersistentStorageUtils struct {
	cache   map[string]UserPersistedValues
	storage IUserPersistentStorage
	active  bool
}

func newUserPersistentStorageUtils(options *Options) *userPersistentStorageUtils {
	return &userPersistentStorageUtils{
		cache:   make(map[string]UserPersistedValues),
		storage: options.UserPersistentStorage,
		active:  options.UserPersistentStorage != nil,
	}
}

func (p *userPersistentStorageUtils) getUserPersistedValues(user User, idType string) UserPersistedValues {
	key := getStorageKey(user, idType)
	if cached, ok := p.cache[key]; ok {
		return cached
	}
	return p.loadFromStorage(key)
}

func (p *userPersistentStorageUtils) loadFromStorage(key string) UserPersistedValues {
	if !p.active {
		return nil
	}

	logError := func(err error) {
		Logger().LogError(fmt.Sprintf("Failed to load key (%s) from UserPersistentStorage (%s)\n", key, err.Error()))
	}

	defer func() {
		if err := recover(); err != nil {
			logError(toError(err))
		}
	}()

	storageValues, exists := p.storage.Load(key)
	if !exists {
		return nil
	}

	var parsedValues UserPersistedValues
	decoder := json.NewDecoder(bytes.NewBufferString(storageValues))
	decoder.UseNumber()
	err := decoder.Decode(&parsedValues)
	if err != nil || parsedValues == nil {
		if err != nil {
			logError(err)
		}
		return nil
	}

	p.cache[key] = parsedValues
	return p.cache[key]
}

func (p *userPersistentStorageUtils) saveToStorage(user User, idType string, userPersistedValues UserPersistedValues) {
	if !p.active {
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

	stringified, err := json.Marshal(userPersistedValues)
	if err != nil {
		logError(err)
		return
	}

	p.storage.Save(key, string(stringified))
}

func (p *userPersistentStorageUtils) removeExperimentFromStorage(user User, idType string, configName string) {
	persistedValues := p.getUserPersistedValues(user, idType)
	if persistedValues != nil {
		delete(persistedValues, configName)
		p.saveToStorage(user, idType, persistedValues)
	}
}

func (p *userPersistentStorageUtils) addEvaluationToUserPersistedValues(userPersistedValues *UserPersistedValues, configName string, evaluation *evalResult) {
	if userPersistedValues == nil {
		*userPersistedValues = make(UserPersistedValues)
	}
	(*userPersistedValues)[configName] = evaluation.toMap()
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
