package status

import (
	"github.com/kyma-project/compass-manager/api/v1beta1"
)

type Status = int

const (
	Registered Status = 1 << iota
	Configured
	Processing
	Failed
	Empty = 0
)

const (
	ReadyState      = "Ready"
	ProcessingState = "Processing"
	FailedState     = "Failed"
)

func StateText(status Status) string {
	if status&Failed != 0 {
		return FailedState
	}

	if status&Processing != 0 {
		return ProcessingState
	}

	if status&(Registered|Configured) == (Registered | Configured) {
		return ReadyState
	}
	return FailedState
}

func StatusNumber(status v1beta1.CompassManagerMappingStatus) Status {
	out := Status(0)

	if status.State == ProcessingState {
		out |= Processing
	}
	if status.State == FailedState {
		out |= Failed
	}

	if status.Registered {
		out |= Registered
	}
	if status.Configured {
		out |= Configured
	}

	return out
}
