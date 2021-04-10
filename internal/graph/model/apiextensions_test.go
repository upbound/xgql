package model

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestGetCompositeResourceDefinition(t *testing.T) {
	schema := `{"cool":true}`

	rschema := runtime.RawExtension{}
	json.Unmarshal([]byte(schema), &rschema)

	cases := map[string]struct {
		reason string
		xrd    *extv1.CompositeResourceDefinition
		want   CompositeResourceDefinition
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			xrd: &extv1.CompositeResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					APIVersion: extv1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
					Kind:       extv1.CompositeResourceDefinitionKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: extv1.CompositeResourceDefinitionSpec{
					Group: "group",
					Names: v1.CustomResourceDefinitionNames{
						Plural:     "clusterexamples",
						Singular:   "clusterexample",
						ShortNames: []string{"cex"},
						Kind:       "ClusterExample",
						ListKind:   "ClusterExampleList",
						Categories: []string{"example"},
					},
					ClaimNames: &v1.CustomResourceDefinitionNames{
						Plural:     "examples",
						Singular:   "example",
						ShortNames: []string{"ex"},
						Kind:       "Example",
						ListKind:   "ExampleList",
						Categories: []string{"example"},
					},
					Versions: []extv1.CompositeResourceDefinitionVersion{{
						Name:          "v1",
						Served:        true,
						Referenceable: true,
						Schema: &extv1.CompositeResourceValidation{
							OpenAPIV3Schema: rschema,
						},
					}},
					DefaultCompositionRef:  &xpv1.Reference{Name: "default"},
					EnforcedCompositionRef: &xpv1.Reference{Name: "enforced"},
				},
				Status: extv1.CompositeResourceDefinitionStatus{
					ConditionedStatus: xpv1.ConditionedStatus{
						Conditions: []xpv1.Condition{{}},
					},
					Controllers: extv1.CompositeResourceDefinitionControllerStatus{
						CompositeResourceTypeRef: extv1.TypeReference{
							APIVersion: "group/v1",
							Kind:       "ClusterExample",
						},
						CompositeResourceClaimTypeRef: extv1.TypeReference{
							APIVersion: "group/v1",
							Kind:       "Example",
						},
					},
				},
			},
			want: CompositeResourceDefinition{
				ID: ReferenceID{
					APIVersion: extv1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
					Kind:       extv1.CompositeResourceDefinitionKind,
					Name:       "cool",
				},
				APIVersion: extv1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
				Kind:       extv1.CompositeResourceDefinitionKind,
				Metadata: &ObjectMeta{
					Name: "cool",
				},
				Spec: &CompositeResourceDefinitionSpec{
					Group: "group",
					Names: &CompositeResourceDefinitionNames{
						Plural:     "clusterexamples",
						Singular:   pointer.StringPtr("clusterexample"),
						ShortNames: []string{"cex"},
						Kind:       "ClusterExample",
						ListKind:   pointer.StringPtr("ClusterExampleList"),
						Categories: []string{"example"},
					},
					ClaimNames: &CompositeResourceDefinitionNames{
						Plural:     "examples",
						Singular:   pointer.StringPtr("example"),
						ShortNames: []string{"ex"},
						Kind:       "Example",
						ListKind:   pointer.StringPtr("ExampleList"),
						Categories: []string{"example"},
					},
					Versions: []CompositeResourceDefinitionVersion{{
						Name:          "v1",
						Referenceable: true,
						Served:        true,
						Schema:        &CompositeResourceValidation{OpenAPIV3Schema: pointer.StringPtr(schema)},
					}},
					DefaultCompositionReference:  &xpv1.Reference{Name: "default"},
					EnforcedCompositionReference: &xpv1.Reference{Name: "enforced"},
				},
				Status: &CompositeResourceDefinitionStatus{
					Conditions: []Condition{{}},
					Controllers: &CompositeResourceDefinitionControllerStatus{
						CompositeResourceType: &TypeReference{
							APIVersion: "group/v1",
							Kind:       "ClusterExample",
						},
						CompositeResourceClaimType: &TypeReference{
							APIVersion: "group/v1",
							Kind:       "Example",
						},
					},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			xrd:    &extv1.CompositeResourceDefinition{},
			want: CompositeResourceDefinition{
				Metadata: &ObjectMeta{},
				Spec: &CompositeResourceDefinitionSpec{
					Names: &CompositeResourceDefinitionNames{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetCompositeResourceDefinition(tc.xrd)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(CompositeResourceDefinition{}, "Raw")); diff != "" {
				t.Errorf("\n%s\nGetCompositeResourceDefinition(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetComposition(t *testing.T) {
	schema := `{"cool":true}`

	rschema := runtime.RawExtension{}
	json.Unmarshal([]byte(schema), &rschema)

	cases := map[string]struct {
		reason string
		xrd    *extv1.Composition
		want   Composition
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			xrd: &extv1.Composition{
				TypeMeta: metav1.TypeMeta{
					APIVersion: extv1.CompositionGroupVersionKind.GroupVersion().String(),
					Kind:       extv1.CompositionKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: extv1.CompositionSpec{
					CompositeTypeRef: extv1.TypeReference{
						APIVersion: "group/v1",
						Kind:       "ClusterExample",
					},
					WriteConnectionSecretsToNamespace: pointer.StringPtr("ns"),
				},
				Status: extv1.CompositionStatus{
					ConditionedStatus: xpv1.ConditionedStatus{
						Conditions: []xpv1.Condition{{}},
					},
				},
			},
			want: Composition{
				ID: ReferenceID{
					APIVersion: extv1.CompositionGroupVersionKind.GroupVersion().String(),
					Kind:       extv1.CompositionKind,
					Name:       "cool",
				},
				APIVersion: extv1.CompositionGroupVersionKind.GroupVersion().String(),
				Kind:       extv1.CompositionKind,
				Metadata: &ObjectMeta{
					Name: "cool",
				},
				Spec: &CompositionSpec{
					CompositeTypeRef: &TypeReference{
						APIVersion: "group/v1",
						Kind:       "ClusterExample",
					},
					WriteConnectionSecretsToNamespace: pointer.StringPtr("ns"),
				},
				Status: &CompositionStatus{
					Conditions: []Condition{{}},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			xrd:    &extv1.Composition{},
			want: Composition{
				Metadata: &ObjectMeta{},
				Spec: &CompositionSpec{
					CompositeTypeRef: &TypeReference{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetComposition(tc.xrd)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Composition{}, "Raw")); diff != "" {
				t.Errorf("\n%s\nGetComposition(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
