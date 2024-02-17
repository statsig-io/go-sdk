package statsig

/**
 * A storage adapter for persisted values. Can be used for sticky bucketing users in experiments.
 */
type IUserPersistentStorage interface {
	/**
	 * Returns the data stored for a specific key
	 */
	Load(key string) (string, bool)

	/**
	 * Updates data stored for a specific key
	 */
	Save(key string, data string)
}
