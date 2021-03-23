package unstructured

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/google/go-cmp/cmp"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ resource.Managed = &Managed{}

func emptyMR() *Managed {
	return &Managed{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{}}}
}

func TestProbablyManaged(t *testing.T) {
	cases := map[string]struct {
		u    *unstructured.Unstructured
		want bool
	}{
		"Probably": {
			u: func() *unstructured.Unstructured {
				o := map[string]interface{}{}
				fieldpath.Pave(o).SetString("spec.providerConfigRef.name", "coolprovider")
				return &unstructured.Unstructured{Object: o}
			}(),
			want: true,
		},
		"ProbablyNot": {
			u: func() *unstructured.Unstructured {
				o := map[string]interface{}{}
				fieldpath.Pave(o).SetValue("spec.providerConfigRef.name", 42) // Not a string.
				return &unstructured.Unstructured{Object: o}
			}(),
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ProbablyManaged(tc.u)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nProbablyManaged(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestManagedCondition(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *Managed
		set    []xpv1.Condition
		get    xpv1.ConditionType
		want   xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to set a condition of an empty Unstructured.",
			u:      emptyMR(),
			set:    []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
			get:    xpv1.TypeReady,
			want:   xpv1.Available(),
		},
		"ExistingCondition": {
			reason: "It should be possible to overwrite a condition that is already set.",
			u: func() *Managed {
				c := emptyMR()
				c.SetConditions(xpv1.Creating())
				return c
			}(),
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Available(),
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &Managed{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Condition{},
		},
		"WeirdStatusConditions": {
			reason: "Conditions should be overwritten if they are not an object.",
			u: &Managed{unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": "wat",
				},
			}}},
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Available(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetConditions(tc.set...)
			got := tc.u.GetCondition(tc.get)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nu.GetCondition(%s): -want, +got:\n%s", tc.reason, tc.get, diff)
			}
		})
	}
}

func TestManagedConditions(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *Managed
		want   []xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to get conditions.",
			u: func() *Managed {
				c := emptyMR()
				c.SetConditions(xpv1.Available(), xpv1.ReconcileSuccess())
				return c
			}(),
			want: []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
		},
		"WeirdStatus": {
			reason: "It should not be possible to get conditions when status is not an object.",
			u: &Managed{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			want: nil,
		},
		"WeirdStatusConditions": {
			reason: "It should notbe possible to get conditions when they are not an array.",
			u: &Managed{unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": "wat",
				},
			}}},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.u.GetConditions()
			if diff := cmp.Diff(tc.want, got, test.EquateConditions()); diff != "" {
				t.Errorf("\n%s\nu.GetConditions(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagedProviderReference(t *testing.T) {
	ref := &xpv1.Reference{Name: "cool"}
	cases := map[string]struct {
		u    *Managed
		set  *xpv1.Reference
		want *xpv1.Reference
	}{
		"NewRef": {
			u:    emptyMR(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetProviderReference(tc.set)
			got := tc.u.GetProviderReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetProviderReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestManagedProviderConfigReference(t *testing.T) {
	ref := &xpv1.Reference{Name: "cool"}
	cases := map[string]struct {
		u    *Managed
		set  *xpv1.Reference
		want *xpv1.Reference
	}{
		"NewRef": {
			u:    emptyMR(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetProviderConfigReference(tc.set)
			got := tc.u.GetProviderConfigReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetProviderConfigReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestManagedWriteConnectionSecretToReference(t *testing.T) {
	ref := &xpv1.SecretReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Managed
		set  *xpv1.SecretReference
		want *xpv1.SecretReference
	}{
		"NewRef": {
			u:    emptyMR(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetWriteConnectionSecretToReference(tc.set)
			got := tc.u.GetWriteConnectionSecretToReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetWriteConnectionSecretToReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestManagedDeletionPolicy(t *testing.T) {
	cases := map[string]struct {
		u    *Managed
		set  xpv1.DeletionPolicy
		want xpv1.DeletionPolicy
	}{
		"NewRef": {
			u:    emptyMR(),
			set:  xpv1.DeletionOrphan,
			want: xpv1.DeletionOrphan,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetDeletionPolicy(tc.set)
			got := tc.u.GetDeletionPolicy()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetDeletionPolicy(): -want, +got:\n%s", diff)
			}
		})
	}
}
