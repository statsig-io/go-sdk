package statsig

// IUserPersistentStorage is a storage adapter for persisted values. Can be used for sticky bucketing users in experiments.
type IUserPersistentStorage interface {
	// Load returns the data stored for a specific key
	Load(key string) (string, bool)

	// Save updates data stored for a specific key
	Save(key string, data string)
}
