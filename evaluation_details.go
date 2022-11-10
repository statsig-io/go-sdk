package statsig

import "time"

type evaluationReason string

const (
	reasonNetwork       evaluationReason = "Network"
	reasonLocalOverride evaluationReason = "LocalOverride"
	reasonDefaultValue  evaluationReason = "DefaultValue"
	reasonUnrecognized  evaluationReason = "Unrecognized"
	reasonUninitialized evaluationReason = "Uninitialized"
	reasonBootstrap     evaluationReason = "Bootstrap"
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
		serverTime:     time.Now().UnixMilli(),
	}
}
