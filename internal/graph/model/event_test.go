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
	kschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
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
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Type:    &warn,
				Reason:  ptr.To("BadStuff"),
				Message: ptr.To("Bad stuff happened."),
				Count:   func() *int { i := 42; return &i }(),
				Source: &EventSource{
					Component: ptr.To("that-thing"),
				},
				FirstTime: &now,
				LastTime:  &now,
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			s:      &corev1.Event{},
			want: Event{
				Metadata:  ObjectMeta{},
				FirstTime: &time.Time{},
				LastTime:  &time.Time{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetEvent(tc.s)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Event{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetEvent(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
