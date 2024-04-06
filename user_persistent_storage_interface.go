package statsig

// The properties of this struct must fit a universal schema that
// when JSON-ified, can be parsed by every SDK supporting user persistent evaluation.
type StickyValues struct {
	Value                         bool                   `json:"value"`
	JsonValue                     map[string]interface{} `json:"json_value"`
	RuleID                        string                 `json:"rule_id"`
	GroupName                     string                 `json:"group_name"`
	SecondaryExposures            []map[string]string    `json:"secondary_exposures"`
	Time                          int64                  `json:"time"`
	ConfigDelegate                string                 `json:"config_delegate,omitempty"`
	ExplicitParameters            map[string]bool        `json:"explicit_parameters,omitempty"`
	UndelegatedSecondaryExposures []map[string]string    `json:"undelegated_secondary_exposures"`
}

type UserPersistedValues = map[string]StickyValues

/**
 * A storage adapter for persisted values. Can be used for sticky bucketing users in experiments.
 */
type IUserPersistentStorage interface {
	/**
	 * Returns the full map of persisted values for a specific user key
	 */
	Load(key string) (UserPersistedValues, bool)

	/**
	 * Save the persisted values of a config given a specific user key
	 */
	Save(key string, configName string, data StickyValues)

	/**
	 * Delete the persisted values of a config given a specific user key
	 */
	Delete(key string, configName string)
}
