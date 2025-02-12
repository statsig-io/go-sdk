package statsig

import "fmt"

type EvaluationSource string

const (
	SourceUninitialized      EvaluationSource = "Uninitialized"
	SourceNetwork            EvaluationSource = "Network"
	SourceNetworkNotModified EvaluationSource = "NetworkNotModified"
	SourceBootstrap          EvaluationSource = "Bootstrap"
	SourceDataAdapter        EvaluationSource = "DataAdapter"
)

type EvaluationReason string

const (
	ReasonNone          EvaluationReason = "None"
	ReasonLocalOverride EvaluationReason = "LocalOverride"
	ReasonUnrecognized  EvaluationReason = "Unrecognized"
	ReasonPersisted     EvaluationReason = "Persisted"
	ReasonUnsupported	EvaluationReason = "Unsupported"
)

type EvaluationDetails struct {
	Source         EvaluationSource
	Reason         EvaluationReason
	ConfigSyncTime int64
	InitTime       int64
	ServerTime     int64
}

func (d EvaluationDetails) detailedReason() string {
	if d.Reason == ReasonNone {
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
		Reason:         ReasonPersisted,
		ConfigSyncTime: configSyncTime,
		InitTime:       0, // unsupported for persisted
		ServerTime:     getUnixMilli(),
	}
}
