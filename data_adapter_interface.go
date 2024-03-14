package statsig

const (
	configSpecsKey = "statsig.cache"
	idListsKey     = "statsig.id_lists"
)

// IDataAdapter is an adapter for implementing custom storage of config specs.
// Can be used to bootstrap Statsig (priority over bootstrapValues if both provided)
// Also useful for backing up cached data
type IDataAdapter interface {
	// Get returns the data stored for a specific key
	Get(key string) string

	// Set updates data stored for each key
	Set(key string, value string)

	// Initialize starts up tasks to run before any get/set calls can be made
	Initialize()

	// Shutdown cleans up tasks to run when statsig is shutdown
	Shutdown()

	// ShouldBeUsedForQueryingUpdates determines whether the SDK should poll for updates
	// from the data adapter (instead of Statsig network) for the given key
	ShouldBeUsedForQueryingUpdates(key string) bool
}
