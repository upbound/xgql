package unstructured

import (
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/google/go-cmp/cmp"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ resource.CompositeClaim = &Claim{}

func emptyXRC() *Claim {
	return &Claim{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{}}}
}
func TestProbablyClaim(t *testing.T) {
	cases := map[string]struct {
		u    *unstructured.Unstructured
		want bool
	}{
		"Probably": {
			u: func() *unstructured.Unstructured {
				o := map[string]interface{}{}
				fieldpath.Pave(o).SetValue("spec.compositionRef", &corev1.ObjectReference{
					Name: "coolcomposition",
				})
				fieldpath.Pave(o).SetValue("spec.resourceRef", &corev1.ObjectReference{
					APIVersion: "example.org/v1",
					Kind:       "Example",
					Name:       "coolexample",
				})
				return &unstructured.Unstructured{Object: o}
			}(),
			want: true,
		},
		"ProbablyNot": {
			u: func() *unstructured.Unstructured {
				o := map[string]interface{}{}
				fieldpath.Pave(o).SetValue("spec.resourceRefs", []string{"wat"}) // Not object refs.
				return &unstructured.Unstructured{Object: o}
			}(),
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ProbablyClaim(tc.u)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nProbablyClaim(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestClaimCondition(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *Claim
		set    []xpv1.Condition
		get    xpv1.ConditionType
		want   xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to set a condition of an empty Claim.",
			u:      emptyXRC(),
			set:    []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
			get:    xpv1.TypeReady,
			want:   xpv1.Available(),
		},
		"ExistingCondition": {
			reason: "It should be possible to overwrite a condition that is already set.",
			u:      emptyXRC(),
			set:    []xpv1.Condition{xpv1.Available()},
			get:    xpv1.TypeReady,
			want:   xpv1.Available(),
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &Claim{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Condition{},
		},
		"WeirdStatusConditions": {
			reason: "Conditions should be overwritten if they are not an object.",
			u: &Claim{unstructured.Unstructured{Object: map[string]interface{}{
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

func TestClaimConditions(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *Claim
		want   []xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to get conditions.",
			u: func() *Claim {
				c := emptyXRC()
				c.SetConditions(xpv1.Available(), xpv1.ReconcileSuccess())
				return c
			}(),
			want: []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
		},
		"WeirdStatus": {
			reason: "It should not be possible to get conditions when status is not an object.",
			u: &Claim{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			want: nil,
		},
		"WeirdStatusConditions": {
			reason: "It should notbe possible to get conditions when they are not an array.",
			u: &Claim{unstructured.Unstructured{Object: map[string]interface{}{
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

func TestCompositionSelector(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"cool": "very"}}
	cases := map[string]struct {
		u    *Claim
		set  *metav1.LabelSelector
		want *metav1.LabelSelector
	}{
		"NewSel": {
			u:    emptyXRC(),
			set:  sel,
			want: sel,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionSelector(tc.set)
			got := tc.u.GetCompositionSelector()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionSelector(): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestCompositionReference(t *testing.T) {
	ref := &corev1.ObjectReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Claim
		set  *corev1.ObjectReference
		want *corev1.ObjectReference
	}{
		"NewRef": {
			u:    emptyXRC(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionReference(tc.set)
			got := tc.u.GetCompositionReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestResourceReference(t *testing.T) {
	ref := &corev1.ObjectReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Claim
		set  *corev1.ObjectReference
		want *corev1.ObjectReference
	}{
		"NewRef": {
			u:    emptyXRC(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetResourceReference(tc.set)
			got := tc.u.GetResourceReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetResourceReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWriteConnectionSecretToReference(t *testing.T) {
	ref := &xpv1.LocalSecretReference{Name: "cool"}
	cases := map[string]struct {
		u    *Claim
		set  *xpv1.LocalSecretReference
		want *xpv1.LocalSecretReference
	}{
		"NewRef": {
			u:    emptyXRC(),
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

func TestConnectionDetailsLastPublishedTime(t *testing.T) {
	now := &metav1.Time{Time: time.Now()}

	// The timestamp loses a little resolution when round-tripped through JSON
	// encoding.
	lores := func(t *metav1.Time) *metav1.Time {
		out := &metav1.Time{}
		j, _ := json.Marshal(t)
		_ = json.Unmarshal(j, out)
		return out
	}

	cases := map[string]struct {
		u    *Claim
		set  *metav1.Time
		want *metav1.Time
	}{
		"NewTime": {
			u:    emptyXRC(),
			set:  now,
			want: lores(now),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetConnectionDetailsLastPublishedTime(tc.set)
			got := tc.u.GetConnectionDetailsLastPublishedTime()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetConnectionDetailsLastPublishedTime(): -want, +got:\n%s", diff)
			}
		})
	}
}
