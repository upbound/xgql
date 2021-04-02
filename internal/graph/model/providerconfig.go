package model

import (
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/upbound/xgql/internal/unstructured"
)

// GetProviderConfig from the suppled Kubernetes ProviderConfig.
func GetProviderConfig(u *kunstructured.Unstructured) ProviderConfig {
	pc := &unstructured.ProviderConfig{Unstructured: *u}
	users := int(pc.GetUsers())
	return ProviderConfig{
		ID: ReferenceID{
			APIVersion: pc.GetAPIVersion(),
			Kind:       pc.GetKind(),
			Name:       pc.GetName(),
		},

		APIVersion: pc.GetAPIVersion(),
		Kind:       pc.GetKind(),
		Metadata:   GetObjectMeta(pc),
		Status: &ProviderConfigStatus{
			Conditions: GetConditions(pc.GetConditions()),
			Users:      &users,
		},
		Raw: raw(pc),
	}
}
