package model

import (
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

// A ManagedResourceSpec specifies the desired state of a managed resource.
type ManagedResourceSpec struct {
	ProviderConfigRef *ProviderConfigReference `json:"providerConfigRef"`
	DeletionPolicy    *DeletionPolicy          `json:"deletionPolicy"`

	WritesConnectionSecretToReference *xpv1.SecretReference
}

// GetDeletionPolicy from the supplied Crossplane policy.
func GetDeletionPolicy(p xpv1.DeletionPolicy) *DeletionPolicy {
	switch p {
	case xpv1.DeletionDelete:
		out := DeletionPolicyDelete
		return &out
	case xpv1.DeletionOrphan:
		out := DeletionPolicyOrphan
		return &out
	default:
		return nil
	}
}

// GetProviderConfigReference from the supplied Crossplane reference.
func GetProviderConfigReference(in *xpv1.Reference) *ProviderConfigReference {
	if in == nil {
		return nil
	}
	return &ProviderConfigReference{Name: in.Name}
}

// GetManagedResourceStatus from the supplied Crossplane resource.
func GetManagedResourceStatus(in *unstructured.Managed) *ManagedResourceStatus {
	c := in.GetConditions()
	if len(c) == 0 {
		return nil
	}
	return &ManagedResourceStatus{Conditions: GetConditions(c)}
}

// GetManagedResource from the supplied Crossplane resource.
func GetManagedResource(u *kunstructured.Unstructured) ManagedResource {
	mg := &unstructured.Managed{Unstructured: *u}
	return ManagedResource{
		ID: ReferenceID{
			APIVersion: mg.GetAPIVersion(),
			Kind:       mg.GetKind(),
			Name:       mg.GetName(),
		},

		APIVersion: mg.GetAPIVersion(),
		Kind:       mg.GetKind(),
		Metadata:   GetObjectMeta(mg),
		Spec: &ManagedResourceSpec{
			WritesConnectionSecretToReference: mg.GetWriteConnectionSecretToReference(),
			ProviderConfigRef:                 GetProviderConfigReference(mg.GetProviderConfigReference()),
			DeletionPolicy:                    GetDeletionPolicy(mg.GetDeletionPolicy()),
		},
		Status:       GetManagedResourceStatus(mg),
		Unstructured: unstruct(mg),
	}
}
