package statsig

import "fmt"

type EvaluationSource string

const (
	sourceUninitialized      EvaluationSource = "Uninitialized"
	sourceNetwork            EvaluationSource = "Network"
	sourceNetworkNotModified EvaluationSource = "NetworkNotModified"
	sourceBootstrap          EvaluationSource = "Bootstrap"
	sourceDataAdapter        EvaluationSource = "DataAdapter"
)

type EvaluationReason string

const (
	reasonNone          EvaluationReason = "None"
	reasonLocalOverride EvaluationReason = "LocalOverride"
	reasonUnrecognized  EvaluationReason = "Unrecognized"
	reasonPersisted     EvaluationReason = "Persisted"
)

type EvaluationDetails struct {
	Source         EvaluationSource
	Reason         EvaluationReason
	ConfigSyncTime int64
	InitTime       int64
	ServerTime     int64
}

func (d EvaluationDetails) detailedReason() string {
	if d.Reason == reasonNone {
		return string(d.Source)
	} else {
		return fmt.Sprintf("%s:%s", d.Source, d.Reason)
	}
}

func newEvaluationDetails(
	source EvaluationSource,
	reason EvaluationReason,
	configSyncTime int64,
	initTime int64,
) *EvaluationDetails {
	return &EvaluationDetails{
		Source:         source,
		Reason:         reason,
		ConfigSyncTime: configSyncTime,
		InitTime:       initTime,
		ServerTime:     getUnixMilli(),
	}
}

func reconstructEvaluationDetailsFromPersisted(
	configSyncTime int64,
) *EvaluationDetails {
	return &EvaluationDetails{
		Reason:         reasonPersisted,
		ConfigSyncTime: configSyncTime,
		InitTime:       0, // unsupported for persisted
		ServerTime:     getUnixMilli(),
	}
}
