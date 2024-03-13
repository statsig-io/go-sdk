package statsig

type evaluationReason string

const (
	reasonNetwork            evaluationReason = "Network"
	reasonBootstrap          evaluationReason = "Bootstrap"
	reasonLocalOverride      evaluationReason = "LocalOverride"
	reasonUnrecognized       evaluationReason = "Unrecognized"
	reasonUninitialized      evaluationReason = "Uninitialized"
	reasonDataAdapter        evaluationReason = "DataAdapter"
	reasonNetworkNotModified evaluationReason = "NetworkNotModified"
	reasonPersisted          evaluationReason = "Persisted"
)

type evaluationDetails struct {
	reason         evaluationReason
	configSyncTime int64
	initTime       int64
	serverTime     int64
}

func newEvaluationDetails(
	reason evaluationReason,
	configSyncTime int64,
	initTime int64,
) *evaluationDetails {
	return &evaluationDetails{
		reason:         reason,
		configSyncTime: configSyncTime,
		initTime:       initTime,
		serverTime:     getUnixMilli(),
	}
}

func reconstructEvaluationDetailsFromPersisted(
	reason evaluationReason,
	configSyncTime int64,
) *evaluationDetails {
	return &evaluationDetails{
		reason:         reason,
		configSyncTime: configSyncTime,
		initTime:       0, // unsupported for persisted
		serverTime:     getUnixMilli(),
	}
}
