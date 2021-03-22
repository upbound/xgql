package model

import xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

// A ManagedResourceSpec specifies the desired state of a managed resource.
type ManagedResourceSpec struct {
	WritesConnectionSecretToRef *xpv1.SecretReference
	ProviderConfigRef           *xpv1.Reference
	DeletionPolicy              *DeletionPolicy `json:"deletionPolicy"`
}

// GetDeletionPolicy from the supplied Crossplane policy.
func GetDeletionPolicy(p xpv1.DeletionPolicy) *DeletionPolicy {
	out := DeletionPolicy(p)
	return &out
}
