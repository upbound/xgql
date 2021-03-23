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

var _ resource.ProviderConfig = &ProviderConfig{}

func emptyPC() *ProviderConfig {
	return &ProviderConfig{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{}}}
}

func TestProbablyProviderConfig(t *testing.T) {
	cases := map[string]struct {
		u    *unstructured.Unstructured
		want bool
	}{
		"Probably": {
			u: func() *unstructured.Unstructured {
				o := map[string]interface{}{}
				fieldpath.Pave(o).SetString("spec.credentials.source", "Secret")
				return &unstructured.Unstructured{Object: o}
			}(),
			want: true,
		},
		"ProbablyNot": {
			u: func() *unstructured.Unstructured {
				o := map[string]interface{}{}
				fieldpath.Pave(o).SetValue("spec.credentials.source", 42) // Not a string.
				return &unstructured.Unstructured{Object: o}
			}(),
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ProbablyProviderConfig(tc.u)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nProbablyProviderConfig(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestProviderConfigUsers(t *testing.T) {
	users := int64(42)
	cases := map[string]struct {
		u    *ProviderConfig
		set  int64
		want int64
	}{
		"NewRef": {
			u:    emptyPC(),
			set:  users,
			want: users,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetUsers(tc.set)
			got := tc.u.GetUsers()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetUsers(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestProviderConfigCondition(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *ProviderConfig
		set    []xpv1.Condition
		get    xpv1.ConditionType
		want   xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to set a condition of an emptyPC Unstructured.",
			u:      emptyPC(),
			set:    []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
			get:    xpv1.TypeReady,
			want:   xpv1.Available(),
		},
		"ExistingCondition": {
			reason: "It should be possible to overwrite a condition that is already set.",
			u: func() *ProviderConfig {
				c := emptyPC()
				c.SetConditions(xpv1.Creating())
				return c
			}(),
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Available(),
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &ProviderConfig{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Condition{},
		},
		"WeirdStatusConditions": {
			reason: "Conditions should be overwritten if they are not an object.",
			u: &ProviderConfig{unstructured.Unstructured{Object: map[string]interface{}{
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

func TestProviderConfigConditions(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *ProviderConfig
		want   []xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to get conditions.",
			u: func() *ProviderConfig {
				c := emptyPC()
				c.SetConditions(xpv1.Available(), xpv1.ReconcileSuccess())
				return c
			}(),
			want: []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
		},
		"WeirdStatus": {
			reason: "It should not be possible to get conditions when status is not an object.",
			u: &ProviderConfig{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			want: nil,
		},
		"WeirdStatusConditions": {
			reason: "It should notbe possible to get conditions when they are not an array.",
			u: &ProviderConfig{unstructured.Unstructured{Object: map[string]interface{}{
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
