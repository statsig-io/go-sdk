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

type EvaluationDetails struct {
	Reason         evaluationReason
	ConfigSyncTime int64
	InitTime       int64
	ServerTime     int64
}

func newEvaluationDetails(
	reason evaluationReason,
	configSyncTime int64,
	initTime int64,
) *EvaluationDetails {
	return &EvaluationDetails{
		Reason:         reason,
		ConfigSyncTime: configSyncTime,
		InitTime:       initTime,
		ServerTime:     getUnixMilli(),
	}
}

func reconstructEvaluationDetailsFromPersisted(
	reason evaluationReason,
	configSyncTime int64,
) *EvaluationDetails {
	return &EvaluationDetails{
		Reason:         reason,
		ConfigSyncTime: configSyncTime,
		InitTime:       0, // unsupported for persisted
		ServerTime:     getUnixMilli(),
	}
}
