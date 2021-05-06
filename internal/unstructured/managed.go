package unstructured

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// ProbablyManaged returns true if the supplied *Unstructured is probably a
// managed resource. It considers any cluster scoped resource with a string
// value at field path spec.providerConfigRef.name to probably be a managed
// resource. spec.providerConfigRef is technically optional, but is defaulted at
// create time by the CRD's OpenAPI schema.
func ProbablyManaged(u *unstructured.Unstructured) bool {
	if u.GetNamespace() != "" {
		return false
	}

	_, err := fieldpath.Pave(u.Object).GetString("spec.providerConfigRef.name")
	return err == nil
}

// A Managed resource.
type Managed struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (u *Managed) GetUnstructured() *unstructured.Unstructured {
	return &u.Unstructured
}

// GetProviderReference of this managed resource.
func (u *Managed) GetProviderReference() *xpv1.Reference {
	out := &xpv1.Reference{}
	if err := fieldpath.Pave(u.Object).GetValueInto("spec.providerRef", out); err != nil {
		return nil
	}
	return out
}

// SetProviderReference of this managed resource.
func (u *Managed) SetProviderReference(ref *xpv1.Reference) {
	_ = fieldpath.Pave(u.Object).SetValue("spec.providerRef", ref)
}

// GetProviderConfigReference of this managed resource.
func (u *Managed) GetProviderConfigReference() *xpv1.Reference {
	out := &xpv1.Reference{}
	if err := fieldpath.Pave(u.Object).GetValueInto("spec.providerConfigRef", out); err != nil {
		return nil
	}
	return out
}

// SetProviderConfigReference of this managed resource.
func (u *Managed) SetProviderConfigReference(ref *xpv1.Reference) {
	_ = fieldpath.Pave(u.Object).SetValue("spec.providerConfigRef", ref)
}

// GetWriteConnectionSecretToReference of this managed resource.
func (u *Managed) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	if err := fieldpath.Pave(u.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this managed resource.
func (u *Managed) SetWriteConnectionSecretToReference(ref *xpv1.SecretReference) {
	_ = fieldpath.Pave(u.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetDeletionPolicy of this managed resource.
func (u *Managed) GetDeletionPolicy() xpv1.DeletionPolicy {
	// The default
	p := xpv1.DeletionDelete
	_ = fieldpath.Pave(u.Object).GetValueInto("spec.deletionPolicy", &p)
	return p
}

// SetDeletionPolicy of this managed resource.
func (u *Managed) SetDeletionPolicy(p xpv1.DeletionPolicy) {
	_ = fieldpath.Pave(u.Object).SetValue("spec.deletionPolicy", &p)
}

// GetCondition of this managed resource.
func (u *Managed) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(u.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this managed resource.
func (u *Managed) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(u.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(u.Object).SetValue("status.conditions", conditioned.Conditions)
}

// NOTE(negz): The below method isn't part of the resource.Managed interface; it
// exists to allow us to extract conditions to convert to our GraphQL model.

// GetConditions of this managed resource.
func (u *Managed) GetConditions() []xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(u.Object).GetValueInto("status", &conditioned); err != nil {
		return nil
	}
	return conditioned.Conditions
}
