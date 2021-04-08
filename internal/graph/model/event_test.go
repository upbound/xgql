package model

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

func TestGetEvent(t *testing.T) {
	now := time.Now()
	warn := EventTypeWarning

	cases := map[string]struct {
		reason string
		s      *corev1.Event
		want   Event
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			s: &corev1.Event{
				TypeMeta: metav1.TypeMeta{
					APIVersion: kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
					Kind:       "Event",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Type:    "Warning",
				Reason:  "BadStuff",
				Message: "Bad stuff happened.",
				Count:   42,
				Source: corev1.EventSource{
					Component: "that-thing",
				},
				FirstTimestamp: metav1.Time{Time: now},
				LastTimestamp:  metav1.Time{Time: now},
			},
			want: Event{
				ID: ReferenceID{
					APIVersion: kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
					Kind:       "Event",
					Name:       "cool",
				},
				APIVersion: kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
				Kind:       "Event",
				Metadata: &ObjectMeta{
					Name: "cool",
				},
				Type:    &warn,
				Reason:  pointer.StringPtr("BadStuff"),
				Message: pointer.StringPtr("Bad stuff happened."),
				Count:   func() *int { i := 42; return &i }(),
				Source: &EventSource{
					Component: pointer.StringPtr("that-thing"),
				},
				FirstTime: &now,
				LastTime:  &now,
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			s:      &corev1.Event{},
			want: Event{
				Metadata:  &ObjectMeta{},
				FirstTime: &time.Time{},
				LastTime:  &time.Time{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetEvent(tc.s)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Event{}, "Raw")); diff != "" {
				t.Errorf("\n%s\nGetEvent(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
