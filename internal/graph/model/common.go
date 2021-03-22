package model

import (
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"

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

// GetLabelSelector from the supplied Kubernetes label selector
func GetLabelSelector(s *metav1.LabelSelector) *LabelSelector {
	if s == nil {
		return nil
	}

	ml := map[string]interface{}{}
	for k, v := range s.MatchLabels {
		ml[k] = v
	}

	return &LabelSelector{MatchLabels: ml}
}

// GetCustomResourceDefinitionVersions from the supplied Kubernetes versions.
func GetCustomResourceDefinitionVersions(in []kextv1.CustomResourceDefinitionVersion) []CustomResourceDefinitionVersion {
	if in == nil {
		return nil
	}

	out := make([]CustomResourceDefinitionVersion, len(in))
	for i := range in {
		out[i] = CustomResourceDefinitionVersion{
			Name:   in[i].Name,
			Served: in[i].Served,
		}

		if s := in[i].Schema; s != nil && s.OpenAPIV3Schema != nil {
			if raw, err := json.Marshal(s.OpenAPIV3Schema); err == nil {
				schema := string(raw)
				out[i].Schema = &CustomResourceValidation{OpenAPIV3Schema: &schema}
			}
		}
	}
	return out
}

// GetCustomResourceDefinitionConditions from the supplied Kubernetes CRD
// conditions.
func GetCustomResourceDefinitionConditions(in []kextv1.CustomResourceDefinitionCondition) []Condition {
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
			Reason:             c.Reason,
			Message:            &c.Message,
		}
	}
	return out
}
