package model

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestGetObjectMeta(t *testing.T) {
	created := time.Now()
	deleted := time.Now()
	md := metav1.NewTime(deleted)

	cases := map[string]struct {
		reason string
		o      metav1.Object
		want   *ObjectMeta
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
			want: &ObjectMeta{
				Namespace:       pointer.StringPtr("default"),
				Name:            "cool-rando",
				GenerateName:    pointer.StringPtr("cool-"),
				UID:             "no-you-id",
				ResourceVersion: "42",
				Generation:      42,
				CreationTime:    created,
				DeletionTime:    &deleted,
				Labels:          map[string]string{"cool": "very"},
				Annotations:     map[string]string{"cool": "very"},
				OwnerReferences: []metav1.OwnerReference{{Name: "owner"}},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			o:      &unstructured.Unstructured{},
			want:   &ObjectMeta{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetObjectMeta(tc.o)
			// metav1.Time trims timestamps to second resolution.
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
				t.Errorf("\n%s\nGetObjectMeta(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
