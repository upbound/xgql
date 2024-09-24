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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestGetObjectMeta(t *testing.T) {
	created := time.Now()
	deleted := time.Now()
	md := metav1.NewTime(deleted)

	cases := map[string]struct {
		reason string
		o      metav1.Object
		want   ObjectMeta
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			o: func() *unstructured.Unstructured {
				u := &unstructured.Unstructured{}

				u.SetNamespace("default")
				u.SetName("cool-rando")
				u.SetGenerateName("cool-")
				u.SetUID(types.UID("no-you-id"))
				u.SetResourceVersion("42")
				u.SetGeneration(42)
				u.SetCreationTimestamp(metav1.NewTime(created))
				u.SetDeletionTimestamp(&md)
				u.SetLabels(map[string]string{"cool": "very"})
				u.SetAnnotations(map[string]string{"cool": "very"})
				u.SetOwnerReferences([]metav1.OwnerReference{{Name: "owner"}})

				return u
			}(),
			want: ObjectMeta{
				Namespace:       ptr.To("default"),
				Name:            "cool-rando",
				GenerateName:    ptr.To("cool-"),
				UID:             "no-you-id",
				ResourceVersion: "42",
				Generation:      42,
				CreationTime:    created,
				DeletionTime:    &deleted,
				OwnerReferences: []metav1.OwnerReference{{Name: "owner"}},
				labels:          map[string]string{"cool": "very"},
				annotations:     map[string]string{"cool": "very"},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			o:      &unstructured.Unstructured{},
			want:   ObjectMeta{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetObjectMeta(tc.o)
			// metav1.Time trims timestamps to second resolution.
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateApproxTime(1*time.Second), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetObjectMeta(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestObjectMetaLabels(t *testing.T) {
	l := map[string]string{
		"some":   "data",
		"more":   "datas",
		"somuch": "ofthedata",
	}

	cases := map[string]struct {
		reason string
		om     ObjectMeta
		keys   []string
		want   map[string]string
	}{
		"NilData": {
			reason: "If no labels exists no labels should be returned.",
			om:     ObjectMeta{},
			keys:   []string{"dataplz"},
			want:   nil,
		},
		"AllData": {
			reason: "If no keys are passed no labels should be returned.",
			om:     ObjectMeta{labels: l},
			want:   l,
		},
		"SomeData": {
			reason: "If keys are passed only those keys (if they exist) should be returned.",
			om:     ObjectMeta{labels: l},
			keys:   []string{"some", "dataplz"},
			want:   map[string]string{"some": "data"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.om.Labels(tc.keys)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nom.Labels(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestObjectMetaAnnotations(t *testing.T) {
	a := map[string]string{
		"some":   "data",
		"more":   "datas",
		"somuch": "ofthedata",
	}

	cases := map[string]struct {
		reason string
		om     ObjectMeta
		keys   []string
		want   map[string]string
	}{
		"NilData": {
			reason: "If no annotations exists no annotations should be returned.",
			om:     ObjectMeta{},
			keys:   []string{"dataplz"},
			want:   nil,
		},
		"AllData": {
			reason: "If no keys are passed no annotations should be returned.",
			om:     ObjectMeta{annotations: a},
			want:   a,
		},
		"SomeData": {
			reason: "If keys are passed only those keys (if they exist) should be returned.",
			om:     ObjectMeta{annotations: a},
			keys:   []string{"some", "dataplz"},
			want:   map[string]string{"some": "data"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.om.Annotations(tc.keys)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nom.Annotations(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
