package model

import (
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/pkg/errors"

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

// GetGenericResource from the suppled Kubernetes resource.
func GetGenericResource(u *kunstructured.Unstructured) (GenericResource, error) {
	raw, err := json.Marshal(u)
	if err != nil {
		return GenericResource{}, errors.Wrap(err, "cannot marshal JSON")
	}

	out := GenericResource{
		APIVersion: u.GetAPIVersion(),
		Kind:       u.GetKind(),
		Metadata:   GetObjectMeta(u),
		Raw:        string(raw),
	}

	return out, nil
}

// GetSecret from the suppled Kubernetes Secret
func GetSecret(s *corev1.Secret) (Secret, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return Secret{}, errors.Wrap(err, "cannot marshal JSON")
	}

	out := Secret{
		APIVersion: s.APIVersion,
		Kind:       s.Kind,
		Metadata:   GetObjectMeta(s),
		Raw:        string(raw),
	}

	return out, nil
}

// GetCustomResourceDefinition from the suppled Kubernetes CRD.
func GetCustomResourceDefinition(crd *extv1.CustomResourceDefinition) (CustomResourceDefinition, error) {
	raw, err := json.Marshal(crd)
	if err != nil {
		return CustomResourceDefinition{}, errors.Wrap(err, "cannot marshal JSON")
	}

	out := CustomResourceDefinition{
		APIVersion: crd.APIVersion,
		Kind:       crd.Kind,
		Metadata:   GetObjectMeta(crd),
		Spec: &CustomResourceDefinitionSpec{
			Group: crd.Spec.Group,
			Names: &CustomResourceDefinitionNames{
				Plural:     crd.Spec.Names.Plural,
				Singular:   &crd.Spec.Names.Singular,
				ShortNames: crd.Spec.Names.ShortNames,
				Kind:       crd.Spec.Names.Kind,
				ListKind:   &crd.Spec.Names.ListKind,
				Categories: crd.Spec.Names.Categories,
			},
			Versions: GetCustomResourceDefinitionVersions(crd.Spec.Versions),
		},
		Status: &CustomResourceDefinitionStatus{
			Conditions: GetCustomResourceDefinitionConditions(crd.Status.Conditions),
		},
		Raw: string(raw),
	}

	return out, nil
}

func getIntPtr(i *int64) *int {
	if i == nil {
		return nil
	}

	out := int(*i)
	return &out
}
