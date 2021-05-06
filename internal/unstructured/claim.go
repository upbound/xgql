package unstructured

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// TODO(negz): This is mostly straight from crossplane-runtime; dedupe with
// github.com/crossplane-runtime/pkg/resource/unstructured/composite.

// ProbablyClaim returns true if the supplied *Unstructured is probably a
// composite resource claim. It considers any namespaced resource with at least
// one of the fields we inject into the OpenAPI schema of resource claims set.
// Note that it is possible for this to produce a false negative. All of these
// injected fields are optional so it's possible that a claim has none of them
// set. Such a claim would not be functional, as indicated by not having a
// (composite) resource ref set.
func ProbablyClaim(u *unstructured.Unstructured) bool {

	p := fieldpath.Pave(u.Object)
	switch {
	case u.GetNamespace() == "":
		return false
	case p.GetValueInto("spec.compositionRef", &corev1.ObjectReference{}) == nil:
		return true
	case p.GetValueInto("spec.compositionSelector", &metav1.LabelSelector{}) == nil:
		return true
	case p.GetValueInto("spec.resourceRef", &corev1.ObjectReference{}) == nil:
		return true
	case p.GetValueInto("spec.writeConnectionSecretToRef", &xpv1.LocalSecretReference{}) == nil:
		return true
	}

	return false
}

// An Claim composite resource claim.
type Claim struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Claim) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this composite resource claim.
func (c *Claim) GetCompositionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector of this composite resource claim.
func (c *Claim) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference of this composite resource claim.
func (c *Claim) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference of this composite resource claim.
func (c *Claim) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetResourceReference of this composite resource claim.
func (c *Claim) GetResourceReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.resourceRef", out); err != nil {
		return nil
	}
	return out
}

// SetResourceReference of this composite resource claim.
func (c *Claim) SetResourceReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRef", ref)
}

// GetWriteConnectionSecretToReference of this composite resource claim.
func (c *Claim) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	out := &xpv1.LocalSecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this composite resource claim.
func (c *Claim) SetWriteConnectionSecretToReference(ref *xpv1.LocalSecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this composite resource claim.
func (c *Claim) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this composite resource claim.
func (c *Claim) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetConnectionDetailsLastPublishedTime of this composite resource claim.
func (c *Claim) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	out := &metav1.Time{}
	if err := fieldpath.Pave(c.Object).GetValueInto("status.connectionDetails.lastPublishedTime", out); err != nil {
		return nil
	}
	return out
}

// SetConnectionDetailsLastPublishedTime of this composite resource claim.
func (c *Claim) SetConnectionDetailsLastPublishedTime(t *metav1.Time) {
	_ = fieldpath.Pave(c.Object).SetValue("status.connectionDetails.lastPublishedTime", t)
}

// NOTE(negz): The below method isn't part of the resource.CompositeClaim
// interface; it exists to allow us to extract conditions to convert to our
// GraphQL model.

// GetConditions of this composite resource claim.
func (c *Claim) GetConditions() []xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return nil
	}
	return conditioned.Conditions
}
