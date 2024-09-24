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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"

	"github.com/upbound/xgql/internal/unstructured"
)

func TestGetCompositeResource(t *testing.T) {
	pub := time.Now()
	mp := metav1.NewTime(pub)

	cases := map[string]struct {
		reason string
		u      *kunstructured.Unstructured
		want   CompositeResource
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			u: func() *kunstructured.Unstructured {
				xr := &unstructured.Composite{Unstructured: kunstructured.Unstructured{}}

				xr.SetAPIVersion("example.org/v1")
				xr.SetKind("CompositeResource")
				xr.SetName("cool")
				xr.SetCompositionSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"cool": "very"}})
				xr.SetCompositionReference(&corev1.ObjectReference{Name: "coolcmp"})
				xr.SetClaimReference(&claim.Reference{Name: "coolclaim"})
				xr.SetResourceReferences([]corev1.ObjectReference{{Name: "coolmanaged"}})
				xr.SetWriteConnectionSecretToReference(&xpv1.SecretReference{Name: "coolsecret"})
				xr.SetConnectionDetailsLastPublishedTime(&mp)
				xr.SetConditions(xpv1.Condition{})

				return xr.GetUnstructured()
			}(),
			want: CompositeResource{
				ID: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "CompositeResource",
					Name:       "cool",
				},
				APIVersion: "example.org/v1",
				Kind:       "CompositeResource",
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: CompositeResourceSpec{
					CompositionSelector:              &LabelSelector{MatchLabels: map[string]string{"cool": "very"}},
					CompositionReference:             &corev1.ObjectReference{Name: "coolcmp"},
					ClaimReference:                   &claim.Reference{Name: "coolclaim"},
					ResourceReferences:               []corev1.ObjectReference{{Name: "coolmanaged"}},
					WriteConnectionSecretToReference: &xpv1.SecretReference{Name: "coolsecret"},
				},
				Status: &CompositeResourceStatus{
					Conditions: []Condition{{}},
					ConnectionDetails: &CompositeResourceConnectionDetails{
						LastPublishedTime: &pub,
					},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			u:      &kunstructured.Unstructured{Object: make(map[string]interface{})},
			want: CompositeResource{
				Metadata: ObjectMeta{},
				Spec: CompositeResourceSpec{
					// We don't mind this empty list being here because it's
					// not exposed as part of our GraphQL API. We use it instead
					// to resolve the resources array.
					ResourceReferences: []corev1.ObjectReference{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetCompositeResource(tc.u)

			// metav1.Time trims timestamps to second resolution.
			if diff := cmp.Diff(tc.want, got,
				cmpopts.IgnoreFields(CompositeResource{}, "PavedAccess"),
				cmpopts.EquateApproxTime(1*time.Second),
				cmp.AllowUnexported(ObjectMeta{}),
			); diff != "" {
				t.Errorf("\n%s\nGetCompositeResource(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetCompositeResourceClaim(t *testing.T) {
	pub := time.Now()
	mp := metav1.NewTime(pub)

	cases := map[string]struct {
		reason string
		u      *kunstructured.Unstructured
		want   CompositeResourceClaim
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			u: func() *kunstructured.Unstructured {
				xrc := &unstructured.Claim{Unstructured: kunstructured.Unstructured{}}

				xrc.SetAPIVersion("example.org/v1")
				xrc.SetKind("CompositeResource")
				xrc.SetNamespace("default")
				xrc.SetName("cool")
				xrc.SetCompositionSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"cool": "very"}})
				xrc.SetCompositionReference(&corev1.ObjectReference{Name: "coolcmp"})
				xrc.SetResourceReference(&corev1.ObjectReference{Name: "coolxr"})
				xrc.SetWriteConnectionSecretToReference(&xpv1.LocalSecretReference{Name: "coolsecret"})
				xrc.SetConnectionDetailsLastPublishedTime(&mp)
				xrc.SetConditions(xpv1.Condition{})

				return xrc.GetUnstructured()
			}(),
			want: CompositeResourceClaim{
				ID: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "CompositeResource",
					Namespace:  "default",
					Name:       "cool",
				},
				APIVersion: "example.org/v1",
				Kind:       "CompositeResource",
				Metadata: ObjectMeta{
					Namespace: ptr.To("default"),
					Name:      "cool",
				},
				Spec: CompositeResourceClaimSpec{
					CompositionSelector:              &LabelSelector{MatchLabels: map[string]string{"cool": "very"}},
					CompositionReference:             &corev1.ObjectReference{Name: "coolcmp"},
					ResourceReference:                &corev1.ObjectReference{Name: "coolxr"},
					WriteConnectionSecretToReference: &xpv1.SecretReference{Namespace: "default", Name: "coolsecret"},
				},
				Status: &CompositeResourceClaimStatus{
					Conditions: []Condition{{}},
					ConnectionDetails: &CompositeResourceClaimConnectionDetails{
						LastPublishedTime: &pub,
					},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			u:      &kunstructured.Unstructured{Object: make(map[string]interface{})},
			want: CompositeResourceClaim{
				Metadata: ObjectMeta{},
				Spec:     CompositeResourceClaimSpec{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetCompositeResourceClaim(tc.u)

			// metav1.Time trims timestamps to second resolution.
			if diff := cmp.Diff(tc.want, got,
				cmpopts.IgnoreFields(CompositeResourceClaim{}, "PavedAccess"),
				cmpopts.EquateApproxTime(1*time.Second),
				cmp.AllowUnexported(ObjectMeta{}),
			); diff != "" {
				t.Errorf("\n%s\nGetCompositeResourceClaim(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
