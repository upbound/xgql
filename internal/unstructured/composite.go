/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unstructured

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
)

// TODO(negz): This is mostly straight from crossplane-runtime; dedupe with
// github.com/crossplane-runtime/pkg/resource/unstructured/composite.

// ProbablyComposite returns true if the supplied *Unstructured is probably a
// composite resource. It considers any cluster scoped resource with at least
// one of the fields we inject into the OpenAPI schema of composite resources set.
// Note that it is possible for this to produce a false negative. All of these
// injected fields are optional so it's possible that an XR has none of them
// set. Such an XR would not be functional, as indicated by not having an array
// of composed resource refs set.
func ProbablyComposite(u *unstructured.Unstructured) bool {
	p := fieldpath.Pave(u.Object)
	r := []corev1.ObjectReference{}
	switch {
	case u.GetNamespace() != "":
		return false
	case p.GetValueInto("spec.compositionRef", &corev1.ObjectReference{}) == nil:
		return true
	case p.GetValueInto("spec.compositionSelector", &metav1.LabelSelector{}) == nil:
		return true
	case p.GetValueInto("spec.resourceRefs", &r) == nil:
		return true
	}

	return false
}

// A Composite resource.
type Composite struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *Unstructured.
func (c *Composite) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this Composite resource.
func (c *Composite) GetCompositionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector of this Composite resource.
func (c *Composite) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference of this Composite resource.
func (c *Composite) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference of this Composite resource.
func (c *Composite) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetClaimReference of this Composite resource.
func (c *Composite) GetClaimReference() *claim.Reference {
	out := &claim.Reference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.claimRef", out); err != nil {
		return nil
	}
	return out
}

// SetClaimReference of this Composite resource.
func (c *Composite) SetClaimReference(ref *claim.Reference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.claimRef", ref)
}

// GetResourceReferences of this Composite resource.
func (c *Composite) GetResourceReferences() []corev1.ObjectReference {
	out := &[]corev1.ObjectReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.resourceRefs", out)
	return *out
}

// SetResourceReferences of this Composite resource.
func (c *Composite) SetResourceReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}
	filtered := make([]corev1.ObjectReference, 0, len(refs))
	for _, ref := range refs {
		// TODO(negz): Ask muvaf to explain what this is working around. :)
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}
		filtered = append(filtered, ref)
	}
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRefs", filtered)
}

// GetWriteConnectionSecretToReference of this Composite resource.
func (c *Composite) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this Composite resource.
func (c *Composite) SetWriteConnectionSecretToReference(ref *xpv1.SecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this Composite resource.
func (c *Composite) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Composite resource.
func (c *Composite) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetConnectionDetailsLastPublishedTime of this Composite resource.
func (c *Composite) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	out := &metav1.Time{}
	if err := fieldpath.Pave(c.Object).GetValueInto("status.connectionDetails.lastPublishedTime", out); err != nil {
		return nil
	}
	return out
}

// SetConnectionDetailsLastPublishedTime of this Composite resource.
func (c *Composite) SetConnectionDetailsLastPublishedTime(t *metav1.Time) {
	_ = fieldpath.Pave(c.Object).SetValue("status.connectionDetails.lastPublishedTime", t)
}

// NOTE(negz): The below method isn't part of the resource.Composite interface;
// it exists to allow us to extract conditions to convert to our GraphQL model.

// GetConditions of this Composite resource.
func (c *Composite) GetConditions() []xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return nil
	}
	return conditioned.Conditions
}

// GetPublishConnectionDetailsTo of this Composite resource.
func (c *Composite) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	out := &xpv1.PublishConnectionDetailsTo{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.publishConnectionDetailsTo", out); err != nil {
		return nil
	}
	return out
}

// SetPublishConnectionDetailsTo of this Composite resource.
func (c *Composite) SetPublishConnectionDetailsTo(ref *xpv1.PublishConnectionDetailsTo) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.publishConnectionDetailsTo", ref)
}

// GetCompositionRevisionReference of this resource claim.
func (c *Composite) GetCompositionRevisionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRevisionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionRevisionReference of this resource claim.
func (c *Composite) SetCompositionRevisionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRevisionRef", ref)
}

// GetCompositionRevisionSelector of this resource claim.
func (c *Composite) GetCompositionRevisionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRevisionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionRevisionSelector of this resource claim.
func (c *Composite) SetCompositionRevisionSelector(ref *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRevisionSelector", ref)
}

// SetCompositionUpdatePolicy of this resource claim.
func (c *Composite) SetCompositionUpdatePolicy(p *xpv1.UpdatePolicy) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionUpdatePolicy", p)
}

// GetCompositionUpdatePolicy of this resource claim.
func (c *Composite) GetCompositionUpdatePolicy() *xpv1.UpdatePolicy {
	p, err := fieldpath.Pave(c.Object).GetString("spec.compositionUpdatePolicy")
	if err != nil {
		return nil
	}
	out := xpv1.UpdatePolicy(p)
	return &out
}

// GetEnvironmentConfigReferences of this Composite resource.
func (c *Composite) GetEnvironmentConfigReferences() []corev1.ObjectReference {
	out := &[]corev1.ObjectReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.environmentConfigRefs", out)
	return *out
}

// SetEnvironmentConfigReferences of this Composite resource.
func (c *Composite) SetEnvironmentConfigReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}
	filtered := make([]corev1.ObjectReference, 0, len(refs))
	for _, ref := range refs {
		// TODO(negz): Ask muvaf to explain what this is working around. :)
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}
		filtered = append(filtered, ref)
	}
	_ = fieldpath.Pave(c.Object).SetValue("spec.environmentConfigRefs", filtered)
}

// SetObservedGeneration of this composite resource claim.
func (c *Composite) SetObservedGeneration(generation int64) {
	status := &xpv1.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)
	status.SetObservedGeneration(generation)
	_ = fieldpath.Pave(c.Object).SetValue("status.observedGeneration", status.ObservedGeneration)
}

// GetObservedGeneration of this composite resource claim.
func (c *Composite) GetObservedGeneration() int64 {
	status := &xpv1.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)
	return status.GetObservedGeneration()
}
