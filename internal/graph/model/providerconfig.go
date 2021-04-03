package model

import (
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/google/go-cmp/cmp"

	"github.com/upbound/xgql/internal/unstructured"
)

// GetProviderConfigStatus from the supplied Crossplane status.
func GetProviderConfigStatus(pc *unstructured.ProviderConfig) *ProviderConfigStatus {
	users := int(pc.GetUsers())
	out := &ProviderConfigStatus{
		Conditions: GetConditions(pc.GetConditions()),
	}
	if users > 0 {
		out.Users = &users
	}
	if cmp.Equal(out, &ProviderConfigStatus{}) {
		return nil
	}
	return out
}

// GetProviderConfig from the suppled Crossplane ProviderConfig.
func GetProviderConfig(u *kunstructured.Unstructured) ProviderConfig {
	pc := &unstructured.ProviderConfig{Unstructured: *u}
	return ProviderConfig{
		ID: ReferenceID{
			APIVersion: pc.GetAPIVersion(),
			Kind:       pc.GetKind(),
			Name:       pc.GetName(),
		},

		APIVersion: pc.GetAPIVersion(),
		Kind:       pc.GetKind(),
		Metadata:   GetObjectMeta(pc),
		Status:     GetProviderConfigStatus(pc),
		Raw:        raw(pc),
	}
}
