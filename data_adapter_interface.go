package statsig

const CONFIG_SPECS_KEY = "statsig.cache"

/**
 * An adapter for implementing custom storage of config specs.
 * Can be used to bootstrap Statsig (priority over bootstrapValues if both provided)
 * Also useful for backing up cached data
 */
type IDataAdapter interface {
	/**
	 * Returns the data stored for a specific key
	 */
	Get(key string) string

	/**
	 * Updates data stored for each key
	 */
	Set(key string, value string)

	/**
	 * Startup tasks to run before any get/set calls can be made
	 */
	Initialize()

	/**
	 * Cleanup tasks to run when statsig is shutdown
	 */
	Shutdown()

	/**
		 * Determines whether the SDK should poll for updates from
	   * the data adapter (instead of Statsig network) for the given key
	*/
	ShouldBeUsedForQueryingUpdates(key string) bool
}
