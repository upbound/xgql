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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

func TestGetConditions(t *testing.T) {
	c := xpv1.Available().WithMessage("I'm here!")
	cases := map[string]struct {
		reason string
		in     []xpv1.Condition
		want   []Condition
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			in:     []xpv1.Condition{c},
			want: []Condition{{
				Type:               string(xpv1.TypeReady),
				Status:             ConditionStatusTrue,
				Reason:             string(xpv1.ReasonAvailable),
				LastTransitionTime: c.LastTransitionTime.Time,
				Message:            ptr.To("I'm here!"),
			}},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			in:     []xpv1.Condition{{}},
			want:   []Condition{{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetConditions(tc.in)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nGetConditions(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetGenericResource(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *kunstructured.Unstructured
		want   GenericResource
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion("example.org/v1")
				u.SetKind("GenericResource")
				u.SetName("cool")
				return u
			}(),
			want: GenericResource{
				ID: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "GenericResource",
					Name:       "cool",
				},
				APIVersion: "example.org/v1",
				Kind:       "GenericResource",
				Metadata: ObjectMeta{
					Name: "cool",
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			u:      &kunstructured.Unstructured{Object: make(map[string]interface{})},
			want: GenericResource{
				Metadata: ObjectMeta{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetGenericResource(tc.u)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(GenericResource{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetGenericResource(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestSecretData(t *testing.T) {
	d := map[string]string{
		"some":   "data",
		"more":   "datas",
		"somuch": "ofthedata",
	}

	cases := map[string]struct {
		reason string
		s      *Secret
		keys   []string
		want   map[string]string
	}{
		"NilData": {
			reason: "If no data exists no data should be returned.",
			s:      &Secret{},
			keys:   []string{"dataplz"},
			want:   nil,
		},
		"AllData": {
			reason: "If no keys are passed no data should be returned.",
			s:      &Secret{data: d},
			want:   d,
		},
		"SomeData": {
			reason: "If keys are passed only those keys (if they exist) should be returned.",
			s:      &Secret{data: d},
			keys:   []string{"some", "dataplz"},
			want:   map[string]string{"some": "data"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.s.Data(tc.keys)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\ns.Data(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetSecret(t *testing.T) {
	cases := map[string]struct {
		reason string
		s      *corev1.Secret
		want   Secret
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			s: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Type: corev1.SecretType("cool"),
				Data: map[string][]byte{"cool": []byte("secret")},
			},
			want: Secret{
				ID: ReferenceID{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
					Name:       "cool",
				},
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Type: ptr.To("cool"),
				data: map[string]string{"cool": "secret"},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			s:      &corev1.Secret{},
			want: Secret{
				ID: ReferenceID{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
				Metadata:   ObjectMeta{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetSecret(tc.s)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Secret{}, "PavedAccess"), cmp.AllowUnexported(Secret{}, ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetSecret(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
func TestConfigMapData(t *testing.T) {
	d := map[string]string{
		"some":   "data",
		"more":   "datas",
		"somuch": "ofthedata",
	}

	cases := map[string]struct {
		reason string
		cm     *ConfigMap
		keys   []string
		want   map[string]string
	}{
		"NilData": {
			reason: "If no data exists no data should be returned.",
			cm:     &ConfigMap{},
			keys:   []string{"dataplz"},
			want:   nil,
		},
		"AllData": {
			reason: "If no keys are passed no data should be returned.",
			cm:     &ConfigMap{data: d},
			want:   d,
		},
		"SomeData": {
			reason: "If keys are passed only those keys (if they exist) should be returned.",
			cm:     &ConfigMap{data: d},
			keys:   []string{"some", "dataplz"},
			want:   map[string]string{"some": "data"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.cm.Data(tc.keys)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\ncm.Data(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetConfigMap(t *testing.T) {
	cases := map[string]struct {
		reason string
		cm     *corev1.ConfigMap
		want   ConfigMap
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			cm: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Data: map[string]string{"cool": "secret"},
			},
			want: ConfigMap{
				ID: ReferenceID{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
					Name:       "cool",
				},
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
				Metadata: ObjectMeta{
					Name: "cool",
				},
				data: map[string]string{"cool": "secret"},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			cm:     &corev1.ConfigMap{},
			want: ConfigMap{
				ID: ReferenceID{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
				Metadata:   ObjectMeta{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetConfigMap(tc.cm)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(ConfigMap{}, "PavedAccess"), cmp.AllowUnexported(ConfigMap{}, ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetSecret(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetCustomResourceDefinition(t *testing.T) {
	schema := &kextv1.JSONSchemaProps{}
	jschema, _ := json.Marshal(schema)
	transition := time.Now()

	cases := map[string]struct {
		reason string
		crd    *unstructured.CustomResourceDefinition
		want   CustomResourceDefinition
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			crd: func() *unstructured.CustomResourceDefinition {
				crd := unstructured.NewCRD()
				crd.SetName("cool")
				crd.SetSpecGroup("group")
				crd.SetSpecNames(kextv1.CustomResourceDefinitionNames{
					Plural:     "clusterexamples",
					Singular:   "clusterexample",
					ShortNames: []string{"cex"},
					Kind:       "ClusterExample",
					ListKind:   "ClusterExampleList",
					Categories: []string{"example"},
				})
				crd.SetSpecScope(kextv1.NamespaceScoped)
				crd.SetSpecVersions([]kextv1.CustomResourceDefinitionVersion{{
					Name:   "v1",
					Served: true,
					Schema: &kextv1.CustomResourceValidation{
						OpenAPIV3Schema: &kextv1.JSONSchemaProps{},
					},
				}})
				crd.SetStatus(kextv1.CustomResourceDefinitionStatus{
					Conditions: []kextv1.CustomResourceDefinitionCondition{{
						Type:               kextv1.Established,
						Reason:             "VeryCoolCRD",
						Message:            "So cool",
						LastTransitionTime: metav1.NewTime(transition),
					}},
				})
				return crd
			}(),
			want: CustomResourceDefinition{
				ID: ReferenceID{
					APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
					Kind:       "CustomResourceDefinition",
					Name:       "cool",
				},
				APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
				Kind:       "CustomResourceDefinition",
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: CustomResourceDefinitionSpec{
					Group: "group",
					Names: CustomResourceDefinitionNames{
						Plural:     "clusterexamples",
						Singular:   ptr.To("clusterexample"),
						ShortNames: []string{"cex"},
						Kind:       "ClusterExample",
						ListKind:   ptr.To("ClusterExampleList"),
						Categories: []string{"example"},
					},
					Scope: ResourceScopeNamespaceScoped,
					Versions: []CustomResourceDefinitionVersion{{
						Name:   "v1",
						Served: true,
						Schema: &CustomResourceValidation{OpenAPIV3Schema: jschema},
					}},
				},
				Status: &CustomResourceDefinitionStatus{
					Conditions: []Condition{{
						Type:    string(kextv1.Established),
						Reason:  "VeryCoolCRD",
						Message: ptr.To("So cool"),
						// NOTE(tnthornton) transition is being truncated
						// during marshaling.
						LastTransitionTime: transition.Truncate(time.Second),
					}},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			crd:    &unstructured.CustomResourceDefinition{},
			want: CustomResourceDefinition{
				ID: ReferenceID{
					APIVersion: "apiextensions.k8s.io/v1",
					Kind:       "CustomResourceDefinition",
				},
				APIVersion: "apiextensions.k8s.io/v1",
				Kind:       "CustomResourceDefinition",
				Metadata:   ObjectMeta{},
				Spec: CustomResourceDefinitionSpec{
					Names:    CustomResourceDefinitionNames{},
					Versions: []CustomResourceDefinitionVersion{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetCustomResourceDefinition(tc.crd)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(CustomResourceDefinition{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetCustomResourceDefinition(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetKubernetesResource(t *testing.T) {
	ignore := []cmp.Option{
		cmp.AllowUnexported(Secret{}, ConfigMap{}, ObjectMeta{}),
		cmpopts.IgnoreFields(ManagedResource{}, "PavedAccess"),
		cmpopts.IgnoreFields(ProviderConfig{}, "PavedAccess"),
		cmpopts.IgnoreFields(CompositeResource{}, "PavedAccess"),
		cmpopts.IgnoreFields(CompositeResourceClaim{}, "PavedAccess"),
		cmpopts.IgnoreFields(Provider{}, "PavedAccess"),
		cmpopts.IgnoreFields(ProviderRevision{}, "PavedAccess"),
		cmpopts.IgnoreFields(Configuration{}, "PavedAccess"),
		cmpopts.IgnoreFields(ConfigurationRevision{}, "PavedAccess"),
		cmpopts.IgnoreFields(CompositeResourceDefinition{}, "PavedAccess"),
		cmpopts.IgnoreFields(Composition{}, "PavedAccess"),
		cmpopts.IgnoreFields(CustomResourceDefinition{}, "PavedAccess"),
		cmpopts.IgnoreFields(Secret{}, "PavedAccess"),
		cmpopts.IgnoreFields(ConfigMap{}, "PavedAccess"),
		cmpopts.IgnoreFields(GenericResource{}, "PavedAccess"),
	}

	dp := DeletionPolicyDelete

	type want struct {
		kr  KubernetesResource
		err error
	}

	cases := map[string]struct {
		u    *kunstructured.Unstructured
		want want
	}{
		"Managed": {
			u: func() *kunstructured.Unstructured {
				// Set a provider ref to convince unstructured.ProbablyManaged
				mg := &unstructured.Managed{}
				mg.SetName("cool")
				mg.SetProviderConfigReference(&xpv1.Reference{Name: "pr"})
				return mg.GetUnstructured()
			}(),
			want: want{
				kr: ManagedResource{
					ID:       ReferenceID{Name: "cool"},
					Metadata: ObjectMeta{Name: "cool"},
					Spec: ManagedResourceSpec{
						ProviderConfigRef: &ProviderConfigReference{Name: "pr"},
						DeletionPolicy:    &dp,
					},
				},
			},
		},
		"ProviderConfig": {
			u: func() *kunstructured.Unstructured {
				pc := &unstructured.ProviderConfig{}
				pc.SetKind("ProviderConfig")
				return pc.GetUnstructured()
			}(),
			want: want{
				kr: ProviderConfig{
					ID:       ReferenceID{Kind: "ProviderConfig"},
					Kind:     "ProviderConfig",
					Metadata: ObjectMeta{},
				},
			},
		},
		"Composite": {
			u: func() *kunstructured.Unstructured {
				// Set resource refs to convince unstructured.ProbablyComposite
				xr := &unstructured.Composite{}
				xr.SetName("cool")
				xr.SetCompositionReference(&corev1.ObjectReference{Name: "cmp"})
				xr.SetResourceReferences([]corev1.ObjectReference{{Name: "cool"}})
				return xr.GetUnstructured()
			}(),
			want: want{
				kr: CompositeResource{
					ID:       ReferenceID{Name: "cool"},
					Metadata: ObjectMeta{Name: "cool"},
					Spec: CompositeResourceSpec{
						CompositionReference: &corev1.ObjectReference{Name: "cmp"},
						ResourceReferences:   []corev1.ObjectReference{{Name: "cool"}},
					},
				},
			},
		},
		"Claim": {
			u: func() *kunstructured.Unstructured {
				// Set resource refs to convince unstructured.ProbablyClaim
				xrc := &unstructured.Claim{}
				xrc.SetNamespace("default")
				xrc.SetName("cool")
				xrc.SetCompositionReference(&corev1.ObjectReference{Name: "cmp"})
				xrc.SetResourceReference(&corev1.ObjectReference{Name: "xr"})
				return xrc.GetUnstructured()
			}(),
			want: want{
				kr: CompositeResourceClaim{
					ID: ReferenceID{
						Namespace: "default",
						Name:      "cool",
					},
					Metadata: ObjectMeta{
						Namespace: ptr.To("default"),
						Name:      "cool",
					},
					Spec: CompositeResourceClaimSpec{
						CompositionReference: &corev1.ObjectReference{Name: "cmp"},
						ResourceReference:    &corev1.ObjectReference{Name: "xr"},
					},
				},
			},
		},
		"Provider": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ProviderKind)
				return u
			}(),
			want: want{
				kr: Provider{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ProviderKind,
					},
					APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ProviderKind,
					Metadata:   ObjectMeta{},
					Spec:       ProviderSpec{},
				},
			},
		},
		"ProviderRevision": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ProviderRevisionKind)
				return u
			}(),
			want: want{
				kr: ProviderRevision{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ProviderRevisionKind,
					},
					APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ProviderRevisionKind,
					Metadata:   ObjectMeta{},
					Spec:       ProviderRevisionSpec{},
				},
			},
		},
		"Configuration": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ConfigurationKind)
				return u
			}(),
			want: want{
				kr: Configuration{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ConfigurationKind,
					},
					APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ConfigurationKind,
					Metadata:   ObjectMeta{},
					Spec:       ConfigurationSpec{},
				},
			},
		},
		"ConfigurationRevision": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ConfigurationRevisionKind)
				return u
			}(),
			want: want{
				kr: ConfigurationRevision{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ConfigurationRevisionKind,
					},
					APIVersion: kschema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ConfigurationRevisionKind,
					Metadata:   ObjectMeta{},
					Spec:       ConfigurationRevisionSpec{},
				},
			},
		},
		"CompositeResourceDefinition": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String())
				u.SetKind(extv1.CompositeResourceDefinitionKind)
				return u
			}(),
			want: want{
				kr: CompositeResourceDefinition{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
						Kind:       extv1.CompositeResourceDefinitionKind,
					},
					APIVersion: kschema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
					Kind:       extv1.CompositeResourceDefinitionKind,
					Metadata:   ObjectMeta{},
					Spec: CompositeResourceDefinitionSpec{
						Names: CompositeResourceDefinitionNames{},
					},
				},
			},
		},
		"Composition": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String())
				u.SetKind(extv1.CompositionKind)
				return u
			}(),
			want: want{
				kr: Composition{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
						Kind:       extv1.CompositionKind,
					},
					APIVersion: kschema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
					Kind:       extv1.CompositionKind,
					Metadata:   ObjectMeta{},
					Spec: CompositionSpec{
						CompositeTypeRef: TypeReference{},
					},
				},
			},
		},
		"CustomResourceDefinition": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String())
				u.SetKind("CustomResourceDefinition")
				return u
			}(),
			want: want{
				kr: CustomResourceDefinition{
					ID: ReferenceID{
						APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
						Kind:       "CustomResourceDefinition",
					},
					APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
					Kind:       "CustomResourceDefinition",
					Metadata:   ObjectMeta{},
					Spec: CustomResourceDefinitionSpec{
						Names:    CustomResourceDefinitionNames{},
						Versions: []CustomResourceDefinitionVersion{},
					},
				},
			},
		},
		"Secret": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String())
				u.SetKind("Secret")
				return u
			}(),
			want: want{
				kr: Secret{
					ID: ReferenceID{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Secret",
					},
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
					Metadata:   ObjectMeta{},
				},
			},
		},
		"ConfigMap": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String())
				u.SetKind("ConfigMap")
				return u
			}(),
			want: want{
				kr: ConfigMap{
					ID: ReferenceID{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					},
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
					Metadata:   ObjectMeta{},
				},
			},
		},
		"Unknown": {
			u: &kunstructured.Unstructured{},
			want: want{
				kr: GenericResource{
					Metadata: ObjectMeta{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kr, err := GetKubernetesResource(tc.u)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("GetKubernetesResource(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.kr, kr, ignore...); diff != "" {
				t.Errorf("GetKubernetesResource(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetObjectReference(t *testing.T) {
	kind := "SomeKind"
	namespace := "some-namespace"
	name := "some-name"

	cases := map[string]struct {
		ref  *corev1.ObjectReference
		want *ObjectReference
	}{
		"Nil": {
			ref:  nil,
			want: nil,
		},
		"JustKind": {
			ref:  &corev1.ObjectReference{Kind: kind},
			want: &ObjectReference{Kind: &kind},
		},
		"JustNamespace": {
			ref:  &corev1.ObjectReference{Namespace: namespace},
			want: &ObjectReference{Namespace: &namespace},
		},
		"JustName": {
			ref:  &corev1.ObjectReference{Name: name},
			want: &ObjectReference{Name: &name},
		},
		"KindAndNamespace": {
			ref:  &corev1.ObjectReference{Kind: kind, Namespace: namespace},
			want: &ObjectReference{Kind: &kind, Namespace: &namespace},
		},
		"KindAndName": {
			ref:  &corev1.ObjectReference{Kind: kind, Name: name},
			want: &ObjectReference{Kind: &kind, Name: &name},
		},
		"NamespaceAndName": {
			ref:  &corev1.ObjectReference{Namespace: namespace, Name: name},
			want: &ObjectReference{Namespace: &namespace, Name: &name},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kr := GetObjectReference(tc.ref)

			if diff := cmp.Diff(tc.want, kr); diff != "" {
				t.Errorf("GetObjectReference(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetSecretReference(t *testing.T) {
	cases := map[string]struct {
		ref  *xpv1.SecretReference
		want *SecretReference
	}{
		"Nil": {
			ref:  nil,
			want: nil,
		},
		"NonNil": {
			ref:  &xpv1.SecretReference{Name: "some-name", Namespace: "some-namespace"},
			want: &SecretReference{Name: "some-name", Namespace: "some-namespace"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kr := GetSecretReference(tc.ref)

			if diff := cmp.Diff(tc.want, kr); diff != "" {
				t.Errorf("GetSecretReference(...): -want, +got:\n%s", diff)
			}
		})
	}
}
