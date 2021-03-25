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
)

// TODO(negz): This is mostly straight from crossplane-runtime; dedupe with
// github.com/crossplane-runtime/pkg/resource/unstructured/composite.

// ProbablyComposite returns true if the supplied *Unstructured is probably a
// composite resource. It considers any resource with an array of object refs at
// spec.resourceRefs to probably be a composite resource.
func ProbablyComposite(u *unstructured.Unstructured) bool {
	r := []corev1.ObjectReference{}
	err := fieldpath.Pave(u.Object).GetValueInto("spec.resourceRefs", &r)
	return err == nil
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
func (c *Composite) GetClaimReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.claimRef", out); err != nil {
		return nil
	}
	return out
}

// SetClaimReference of this Composite resource.
func (c *Composite) SetClaimReference(ref *corev1.ObjectReference) {
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

// GetConditions of this managed resource.
func (c *Composite) GetConditions() []xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return nil
	}
	return conditioned.Conditions
}
