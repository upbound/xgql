package model

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// GetConditions from the supplied Crossplane conditions.
func GetConditions(in []xpv1.Condition) []Condition {
	if in == nil {
		return nil
	}

	out := make([]Condition, len(in))
	for i := range in {
		c := in[i] // So we don't take the address of the range variable.

		out[i] = Condition{
			Type:               string(c.Type),
			Status:             ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime.Time,
			Reason:             string(c.Reason),
			Message:            &c.Message,
		}
	}
	return out
}
