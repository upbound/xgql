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
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestGetCompositeResourceDefinition(t *testing.T) {
	schema := []byte(`{"cool":true}`)

	rschema := runtime.RawExtension{}
	json.Unmarshal(schema, &rschema)

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
					DefaultCompositionRef:  &extv1.CompositionReference{Name: "default"},
					EnforcedCompositionRef: &extv1.CompositionReference{Name: "enforced"},
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
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: CompositeResourceDefinitionSpec{
					Group: "group",
					Names: CompositeResourceDefinitionNames{
						Plural:     "clusterexamples",
						Singular:   ptr.To("clusterexample"),
						ShortNames: []string{"cex"},
						Kind:       "ClusterExample",
						ListKind:   ptr.To("ClusterExampleList"),
						Categories: []string{"example"},
					},
					ClaimNames: &CompositeResourceDefinitionNames{
						Plural:     "examples",
						Singular:   ptr.To("example"),
						ShortNames: []string{"ex"},
						Kind:       "Example",
						ListKind:   ptr.To("ExampleList"),
						Categories: []string{"example"},
					},
					Versions: []CompositeResourceDefinitionVersion{{
						Name:          "v1",
						Referenceable: true,
						Served:        true,
						Schema:        &CompositeResourceValidation{OpenAPIV3Schema: schema},
					}},
					DefaultCompositionReference:  &extv1.CompositionReference{Name: "default"},
					EnforcedCompositionReference: &extv1.CompositionReference{Name: "enforced"},
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
				Metadata: ObjectMeta{},
				Spec: CompositeResourceDefinitionSpec{
					Names: CompositeResourceDefinitionNames{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetCompositeResourceDefinition(tc.xrd)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(CompositeResourceDefinition{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
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
					WriteConnectionSecretsToNamespace: ptr.To("ns"),
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
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: CompositionSpec{
					CompositeTypeRef: TypeReference{
						APIVersion: "group/v1",
						Kind:       "ClusterExample",
					},
					WriteConnectionSecretsToNamespace: ptr.To("ns"),
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			xrd:    &extv1.Composition{},
			want: Composition{
				Metadata: ObjectMeta{},
				Spec: CompositionSpec{
					CompositeTypeRef: TypeReference{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetComposition(tc.xrd)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Composition{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetComposition(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestDefinedCompositeResourceOptionsInputDeprecation(t *testing.T) {
	version1 := "v1"
	version2 := "v2"
	type args struct {
		options *DefinedCompositeResourceOptionsInput
		version *string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   *DefinedCompositeResourceOptionsInput
	}{
		"VersionNone": {
			reason: "All supported fields should be converted to our model",
			args: args{
				options: &DefinedCompositeResourceOptionsInput{Version: nil},
				version: nil,
			},
			want: &DefinedCompositeResourceOptionsInput{},
		},
		"VersionNonDeprecated": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options: &DefinedCompositeResourceOptionsInput{Version: &version1},
				version: nil,
			},
			want: &DefinedCompositeResourceOptionsInput{Version: &version1},
		},
		"VersionDeprecated": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options: &DefinedCompositeResourceOptionsInput{Version: nil},
				version: &version1,
			},
			want: &DefinedCompositeResourceOptionsInput{Version: &version1},
		},
		"VersionBoth": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options: &DefinedCompositeResourceOptionsInput{Version: &version1},
				version: &version2,
			},
			want: &DefinedCompositeResourceOptionsInput{Version: &version1},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.options.DeprecationPatch(tc.args.version)
			if diff := cmp.Diff(tc.want, tc.args.options); diff != "" {
				t.Errorf("\n%s\nDeprecationPatch(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestDefinedCompositeResourceClaimOptionsInputDeprecation(t *testing.T) {
	version1 := "v1"
	version2 := "v2"
	namespace1 := "namespace1"
	namespace2 := "namespace2"
	type args struct {
		options   *DefinedCompositeResourceClaimOptionsInput
		version   *string
		namespace *string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   *DefinedCompositeResourceClaimOptionsInput
	}{
		"VersionNone": {
			reason: "All supported fields should be converted to our model",
			args: args{
				options: &DefinedCompositeResourceClaimOptionsInput{Version: nil},
				version: nil,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{},
		},
		"VersionNonDeprecated": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options: &DefinedCompositeResourceClaimOptionsInput{Version: &version1},
				version: nil,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{Version: &version1},
		},
		"VersionDeprecated": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options: &DefinedCompositeResourceClaimOptionsInput{Version: nil},
				version: &version1,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{Version: &version1},
		},
		"VersionBoth": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options: &DefinedCompositeResourceClaimOptionsInput{Version: &version1},
				version: &version2,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{Version: &version1},
		},
		"NamespaceNone": {
			reason: "All supported fields should be converted to our model",
			args: args{
				options:   &DefinedCompositeResourceClaimOptionsInput{Namespace: nil},
				namespace: nil,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{},
		},
		"NamespaceNonDeprecated": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options:   &DefinedCompositeResourceClaimOptionsInput{Namespace: &namespace1},
				namespace: nil,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{Namespace: &namespace1},
		},
		"NamespaceDeprecated": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options:   &DefinedCompositeResourceClaimOptionsInput{Namespace: nil},
				namespace: &namespace1,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{Namespace: &namespace1},
		},
		"NamespaceBoth": {
			reason: "Absent optional fields should be absent in our model",
			args: args{
				options:   &DefinedCompositeResourceClaimOptionsInput{Namespace: &namespace1},
				namespace: &namespace2,
			},
			want: &DefinedCompositeResourceClaimOptionsInput{Namespace: &namespace1},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.options.DeprecationPatch(tc.args.version, tc.args.namespace)
			if diff := cmp.Diff(tc.want, tc.args.options); diff != "" {
				t.Errorf("\n%s\nDeprecationPatch(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
