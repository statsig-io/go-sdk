package statsig

/**
 * An adapter for implementing custom storage of config specs.
 * Can be used to bootstrap Statsig (priority over bootstrapValues if both provided)
 * Also useful for backing up cached data
 */
type IDataAdapter interface {
	/**
	 * Returns the data stored for a specific key
	 */
	get(key string) string

	/**
	 * Updates data stored for each key
	 */
	set(key string, value string)

	/**
	 * Startup tasks to run before any get/set calls can be made
	 */
	initialize()

	/**
	 * Cleanup tasks to run when statsig is shutdown
	 */
	shutdown()
}
