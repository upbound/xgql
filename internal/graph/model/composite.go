// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"

	"github.com/upbound/xgql/internal/unstructured"
)

// A CompositeResourceSpec defines the desired state of a composite resource.
type CompositeResourceSpec struct {
	CompositionSelector *LabelSelector `json:"compositionSelector"`

	CompositionReference             *corev1.ObjectReference
	ClaimReference                   *claim.Reference
	ResourceReferences               []corev1.ObjectReference
	WriteConnectionSecretToReference *xpv1.SecretReference
}

// GetCompositeResourceStatus from the supplied Crossplane composite.
func GetCompositeResourceStatus(xr *unstructured.Composite) *CompositeResourceStatus {
	c := xr.GetConditions()
	t := xr.GetConnectionDetailsLastPublishedTime()

	out := &CompositeResourceStatus{}
	if len(c) > 0 {
		out.Conditions = GetConditions(c)
	}
	if t != nil {
		out.ConnectionDetails = &CompositeResourceConnectionDetails{LastPublishedTime: &t.Time}
	}

	if cmp.Equal(out, &CompositeResourceStatus{}) {
		return nil
	}

	return out
}

// GetCompositeResource from the supplied Crossplane resource.
func GetCompositeResource(u *kunstructured.Unstructured) CompositeResource {
	xr := &unstructured.Composite{Unstructured: *u}
	return CompositeResource{
		ID: ReferenceID{
			APIVersion: xr.GetAPIVersion(),
			Kind:       xr.GetKind(),
			Name:       xr.GetName(),
		},

		APIVersion: xr.GetAPIVersion(),
		Kind:       xr.GetKind(),
		Metadata:   GetObjectMeta(xr),
		Spec: CompositeResourceSpec{
			CompositionSelector:              GetLabelSelector(xr.GetCompositionSelector()),
			CompositionReference:             xr.GetCompositionReference(),
			ClaimReference:                   xr.GetClaimReference(),
			ResourceReferences:               xr.GetResourceReferences(),
			WriteConnectionSecretToReference: xr.GetWriteConnectionSecretToReference(),
		},
		Status: GetCompositeResourceStatus(xr),
		PavedAccess: PavedAccess{
			Paved: fieldpath.Pave(u.Object),
		},
	}
}

func delocalize(ref *xpv1.LocalSecretReference, namespace string) *xpv1.SecretReference {
	if ref == nil {
		return nil
	}
	return &xpv1.SecretReference{Namespace: namespace, Name: ref.Name}
}

// A CompositeResourceClaimSpec represents the desired state of a composite
// resource claim.
type CompositeResourceClaimSpec struct {
	CompositionSelector *LabelSelector `json:"compositionSelector"`

	CompositionReference *corev1.ObjectReference
	ResourceReference    *corev1.ObjectReference

	// We use a non-local secret reference because we need to know what
	// namespace the secret is in when we're resolving it, when we only have
	// access to the spec.
	WriteConnectionSecretToReference *xpv1.SecretReference
}

// GetCompositeResourceClaimStatus from the supplied Crossplane claim.
func GetCompositeResourceClaimStatus(xrc *unstructured.Claim) *CompositeResourceClaimStatus {
	c := xrc.GetConditions()
	t := xrc.GetConnectionDetailsLastPublishedTime()

	out := &CompositeResourceClaimStatus{}
	if len(c) > 0 {
		out.Conditions = GetConditions(c)
	}
	if t != nil {
		out.ConnectionDetails = &CompositeResourceClaimConnectionDetails{LastPublishedTime: &t.Time}
	}

	if cmp.Equal(out, &CompositeResourceClaimStatus{}) {
		return nil
	}

	return out
}

// GetCompositeResourceClaim from the supplied Crossplane claim.
func GetCompositeResourceClaim(u *kunstructured.Unstructured) CompositeResourceClaim {
	xrc := &unstructured.Claim{Unstructured: *u}
	return CompositeResourceClaim{
		ID: ReferenceID{
			APIVersion: xrc.GetAPIVersion(),
			Kind:       xrc.GetKind(),
			Namespace:  xrc.GetNamespace(),
			Name:       xrc.GetName(),
		},

		APIVersion: xrc.GetAPIVersion(),
		Kind:       xrc.GetKind(),
		Metadata:   GetObjectMeta(xrc),
		Spec: CompositeResourceClaimSpec{
			CompositionSelector:              GetLabelSelector(xrc.GetCompositionSelector()),
			CompositionReference:             xrc.GetCompositionReference(),
			ResourceReference:                xrc.GetResourceReference(),
			WriteConnectionSecretToReference: delocalize(xrc.GetWriteConnectionSecretToReference(), xrc.GetNamespace()),
		},
		Status: GetCompositeResourceClaimStatus(xrc),
		PavedAccess: PavedAccess{
			Paved: fieldpath.Pave(u.Object),
		},
	}
}
