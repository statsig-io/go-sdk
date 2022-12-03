package statsig

type evaluationReason string

const (
	reasonNetwork       evaluationReason = "Network"
	reasonBootstrap     evaluationReason = "Bootstrap"
	reasonLocalOverride evaluationReason = "LocalOverride"
	reasonUnrecognized  evaluationReason = "Unrecognized"
	reasonUninitialized evaluationReason = "Uninitialized"
	reasonDataAdapter   evaluationReason = "DataAdapter"
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
		serverTime:     now().UnixMilli(),
	}
}
