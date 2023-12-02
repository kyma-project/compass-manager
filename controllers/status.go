package controllers

import "github.com/kyma-project/compass-manager/api/v1beta1"

type Status = int

const (
	Registered Status = 1 << iota
	Configured
	Processing
	Failed
)

func stateText(status Status) string {
	if status&Failed != 0 {
		return "Failed"
	}

	if status&Processing != 0 {
		return "Processing"
	}

	if status&(Registered|Configured) == (Registered | Configured) {
		return "Ready"
	}
	return "Failed"
}

func statusNumber(status v1beta1.CompassManagerMappingStatus) Status {
	out := Status(0)

	if status.State == "Processing" {
		out |= Processing
	}
	if status.State == "Failed" {
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
