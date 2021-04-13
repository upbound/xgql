package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

func TestGetManagedResource(t *testing.T) {
	delete := DeletionPolicyDelete
	orphan := DeletionPolicyOrphan

	cases := map[string]struct {
		reason string
		u      *kunstructured.Unstructured
		want   ManagedResource
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			u: func() *kunstructured.Unstructured {
				mr := &unstructured.Managed{Unstructured: kunstructured.Unstructured{}}

				mr.SetAPIVersion("example.org/v1")
				mr.SetKind("ManagedResource")
				mr.SetName("cool")
				mr.SetProviderConfigReference(&xpv1.Reference{Name: "coolprov"})
				mr.SetWriteConnectionSecretToReference(&xpv1.SecretReference{Name: "coolsecret"})
				mr.SetConditions(xpv1.Condition{})
				mr.SetDeletionPolicy(xpv1.DeletionOrphan)

				return mr.GetUnstructured()
			}(),
			want: ManagedResource{
				ID: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "ManagedResource",
					Name:       "cool",
				},
				APIVersion: "example.org/v1",
				Kind:       "ManagedResource",
				Metadata: &ObjectMeta{
					Name: "cool",
				},
				Spec: &ManagedResourceSpec{
					ProviderConfigRef:                 &ProviderConfigReference{Name: "coolprov"},
					DeletionPolicy:                    &orphan,
					WritesConnectionSecretToReference: &xpv1.SecretReference{Name: "coolsecret"},
				},
				Status: &ManagedResourceStatus{
					Conditions: []Condition{{}},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			u:      &kunstructured.Unstructured{Object: make(map[string]interface{})},
			want: ManagedResource{
				Metadata: &ObjectMeta{},
				Spec: &ManagedResourceSpec{
					// This is technically optional, but it's basically always
					// set to 'delete' using CRD defaulting. We also default it
					// to 'delete' in unstructured.Managed.
					DeletionPolicy: &delete,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetManagedResource(tc.u)

			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(ManagedResource{}, "Raw"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetManagedResource(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
