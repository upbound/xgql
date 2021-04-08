package model

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

func TestReferenceID(t *testing.T) {
	cases := map[string]struct {
		reason string
		id     ReferenceID
		want   string
	}{
		"Namespaced": {
			reason: "It should be possible to encode a namespaced ID",
			id: ReferenceID{
				APIVersion: "example.org/v1",
				Kind:       "ExampleKind",
				Namespace:  "default",
				Name:       "example",
			},
			want: "ZXhhbXBsZS5vcmcvdjF8RXhhbXBsZUtpbmR8ZGVmYXVsdHxleGFtcGxl",
		},
		"ClusterScoped": {
			reason: "It should be possible to encode a cluster scoped ID",
			id: ReferenceID{
				APIVersion: "example.org/v1",
				Kind:       "ExampleKind",
				Name:       "example",
			},
			want: "ZXhhbXBsZS5vcmcvdjF8RXhhbXBsZUtpbmR8fGV4YW1wbGU",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.id.String()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nid.String(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParseReferenceID(t *testing.T) {
	_, decodeErr := encoder.DecodeString("=")

	type want struct {
		id  ReferenceID
		err error
	}
	cases := map[string]struct {
		reason string
		id     string
		want   want
	}{
		"Namespaced": {
			reason: "It should be possible to decode a namespaced ID",
			id:     "ZXhhbXBsZS5vcmcvdjF8RXhhbXBsZUtpbmR8ZGVmYXVsdHxleGFtcGxl",
			want: want{
				id: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "ExampleKind",
					Namespace:  "default",
					Name:       "example",
				},
			},
		},
		"ClusterScoped": {
			reason: "It should be possible to decode a cluster scoped ID",
			id:     "ZXhhbXBsZS5vcmcvdjF8RXhhbXBsZUtpbmR8fGV4YW1wbGU",
			want: want{
				id: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "ExampleKind",
					Name:       "example",
				},
			},
		},
		"WrongEncoding": {
			reason: "Attempting to parse an ID that is not base64 encoded should result in an error",
			id:     "=",
			want: want{
				err: errors.Wrap(decodeErr, errDecode),
			},
		},
		"WrongParts": {
			reason: "Attempting to parse a malformed ID should result in an error",
			id:     "cG90YXRvCg",
			want: want{
				err: errors.New(errMalformed),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseReferenceID(tc.id)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParseReferenceID(%q): -want error, +got error:\n%s", tc.reason, tc.id, diff)
			}
			if diff := cmp.Diff(tc.want.id, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParseReferenceID(%q): -want, +got:\n%s", tc.reason, tc.id, diff)
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
				Metadata: &ObjectMeta{
					Name: "cool",
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			u:      &kunstructured.Unstructured{Object: make(map[string]interface{})},
			want: GenericResource{
				Metadata: &ObjectMeta{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetGenericResource(tc.u)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(GenericResource{}, "Raw")); diff != "" {
				t.Errorf("\n%s\nGetGenericResource(...): -want, +got\n:%s", tc.reason, diff)
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
					APIVersion: kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Data: map[string][]byte{"cool": []byte("secret")},
			},
			want: Secret{
				ID: ReferenceID{
					APIVersion: kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
					Kind:       "Secret",
					Name:       "cool",
				},
				APIVersion: kschema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
				Kind:       "Secret",
				Metadata: &ObjectMeta{
					Name: "cool",
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			s:      &corev1.Secret{},
			want: Secret{
				Metadata: &ObjectMeta{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetSecret(tc.s)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Secret{}, "Raw")); diff != "" {
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
		crd    *kextv1.CustomResourceDefinition
		want   CustomResourceDefinition
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			crd: &kextv1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
					Kind:       "CustomResourceDefinition",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: kextv1.CustomResourceDefinitionSpec{
					Group: "group",
					Names: kextv1.CustomResourceDefinitionNames{
						Plural:     "clusterexamples",
						Singular:   "clusterexample",
						ShortNames: []string{"cex"},
						Kind:       "ClusterExample",
						ListKind:   "ClusterExampleList",
						Categories: []string{"example"},
					},
					Versions: []kextv1.CustomResourceDefinitionVersion{{
						Name:   "v1",
						Served: true,
						Schema: &kextv1.CustomResourceValidation{
							OpenAPIV3Schema: &kextv1.JSONSchemaProps{},
						},
					}},
				},
				Status: kextv1.CustomResourceDefinitionStatus{
					Conditions: []kextv1.CustomResourceDefinitionCondition{{
						Type:               kextv1.Established,
						Reason:             "VeryCoolCRD",
						Message:            "So cool",
						LastTransitionTime: metav1.NewTime(transition),
					}},
				},
			},
			want: CustomResourceDefinition{
				ID: ReferenceID{
					APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
					Kind:       "CustomResourceDefinition",
					Name:       "cool",
				},
				APIVersion: kschema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
				Kind:       "CustomResourceDefinition",
				Metadata: &ObjectMeta{
					Name: "cool",
				},
				Spec: &CustomResourceDefinitionSpec{
					Group: "group",
					Names: &CustomResourceDefinitionNames{
						Plural:     "clusterexamples",
						Singular:   pointer.StringPtr("clusterexample"),
						ShortNames: []string{"cex"},
						Kind:       "ClusterExample",
						ListKind:   pointer.StringPtr("ClusterExampleList"),
						Categories: []string{"example"},
					},
					Versions: []CustomResourceDefinitionVersion{{
						Name:   "v1",
						Served: true,
						Schema: &CustomResourceValidation{OpenAPIV3Schema: pointer.StringPtr(string(jschema))},
					}},
				},
				Status: &CustomResourceDefinitionStatus{
					Conditions: []Condition{{
						Type:               string(kextv1.Established),
						Reason:             "VeryCoolCRD",
						Message:            pointer.StringPtr("So cool"),
						LastTransitionTime: transition,
					}},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			crd:    &kextv1.CustomResourceDefinition{},
			want: CustomResourceDefinition{
				Metadata: &ObjectMeta{},
				Spec: &CustomResourceDefinitionSpec{
					Names: &CustomResourceDefinitionNames{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetCustomResourceDefinition(tc.crd)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(CustomResourceDefinition{}, "Raw")); diff != "" {
				t.Errorf("\n%s\nGetCustomResourceDefinition(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetKubernetesResource(t *testing.T) {
	ignore := []cmp.Option{
		cmpopts.IgnoreFields(ManagedResource{}, "Raw"),
		cmpopts.IgnoreFields(ProviderConfig{}, "Raw"),
		cmpopts.IgnoreFields(CompositeResource{}, "Raw"),
		cmpopts.IgnoreFields(CompositeResourceClaim{}, "Raw"),
		cmpopts.IgnoreFields(Provider{}, "Raw"),
		cmpopts.IgnoreFields(ProviderRevision{}, "Raw"),
		cmpopts.IgnoreFields(Configuration{}, "Raw"),
		cmpopts.IgnoreFields(ConfigurationRevision{}, "Raw"),
		cmpopts.IgnoreFields(CompositeResourceDefinition{}, "Raw"),
		cmpopts.IgnoreFields(Composition{}, "Raw"),
		cmpopts.IgnoreFields(CustomResourceDefinition{}, "Raw"),
		cmpopts.IgnoreFields(Secret{}, "Raw"),
		cmpopts.IgnoreFields(GenericResource{}, "Raw"),
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
					Metadata: &ObjectMeta{Name: "cool"},
					Spec: &ManagedResourceSpec{
						ProviderConfigRef: &xpv1.Reference{Name: "pr"},
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
					Metadata: &ObjectMeta{},
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
					Metadata: &ObjectMeta{Name: "cool"},
					Spec: &CompositeResourceSpec{
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
				xrc.SetName("cool")
				xrc.SetCompositionReference(&corev1.ObjectReference{Name: "cmp"})
				xrc.SetResourceReference(&corev1.ObjectReference{Name: "xr"})
				return xrc.GetUnstructured()
			}(),
			want: want{
				kr: CompositeResourceClaim{
					ID:       ReferenceID{Name: "cool"},
					Metadata: &ObjectMeta{Name: "cool"},
					Spec: &CompositeResourceClaimSpec{
						CompositionReference: &corev1.ObjectReference{Name: "cmp"},
						ResourceReference:    &corev1.ObjectReference{Name: "xr"},
					},
				},
			},
		},
		"Provider": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ProviderKind)
				return u
			}(),
			want: want{
				kr: Provider{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ProviderKind,
					},
					APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ProviderKind,
					Metadata:   &ObjectMeta{},
					Spec:       &ProviderSpec{},
				},
			},
		},
		"ProviderRevision": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ProviderRevisionKind)
				return u
			}(),
			want: want{
				kr: ProviderRevision{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ProviderRevisionKind,
					},
					APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ProviderRevisionKind,
					Metadata:   &ObjectMeta{},
					Spec:       &ProviderRevisionSpec{},
				},
			},
		},
		"Configuration": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ConfigurationKind)
				return u
			}(),
			want: want{
				kr: Configuration{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ConfigurationKind,
					},
					APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ConfigurationKind,
					Metadata:   &ObjectMeta{},
					Spec:       &ConfigurationSpec{},
				},
			},
		},
		"ConfigurationRevision": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String())
				u.SetKind(pkgv1.ConfigurationRevisionKind)
				return u
			}(),
			want: want{
				kr: ConfigurationRevision{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
						Kind:       pkgv1.ConfigurationRevisionKind,
					},
					APIVersion: schema.GroupVersion{Group: pkgv1.Group, Version: pkgv1.Version}.String(),
					Kind:       pkgv1.ConfigurationRevisionKind,
					Metadata:   &ObjectMeta{},
					Spec:       &ConfigurationRevisionSpec{},
				},
			},
		},
		"CompositeResourceDefinition": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String())
				u.SetKind(extv1.CompositeResourceDefinitionKind)
				return u
			}(),
			want: want{
				kr: CompositeResourceDefinition{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
						Kind:       extv1.CompositeResourceDefinitionKind,
					},
					APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
					Kind:       extv1.CompositeResourceDefinitionKind,
					Metadata:   &ObjectMeta{},
					Spec: &CompositeResourceDefinitionSpec{
						Names: &CompositeResourceDefinitionNames{},
					},
				},
			},
		},
		"Composition": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String())
				u.SetKind(extv1.CompositionKind)
				return u
			}(),
			want: want{
				kr: Composition{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
						Kind:       extv1.CompositionKind,
					},
					APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
					Kind:       extv1.CompositionKind,
					Metadata:   &ObjectMeta{},
					Spec: &CompositionSpec{
						CompositeTypeRef: &TypeReference{},
					},
				},
			},
		},
		"CustomResourceDefinition": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String())
				u.SetKind("CustomResourceDefinition")
				return u
			}(),
			want: want{
				kr: CustomResourceDefinition{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
						Kind:       "CustomResourceDefinition",
					},
					APIVersion: schema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
					Kind:       "CustomResourceDefinition",
					Metadata:   &ObjectMeta{},
					Spec: &CustomResourceDefinitionSpec{
						Names: &CustomResourceDefinitionNames{},
					},
				},
			},
		},
		"Secret": {
			u: func() *kunstructured.Unstructured {
				u := &kunstructured.Unstructured{}
				u.SetAPIVersion(schema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String())
				u.SetKind("Secret")
				return u
			}(),
			want: want{
				kr: Secret{
					ID: ReferenceID{
						APIVersion: schema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
						Kind:       "Secret",
					},
					APIVersion: schema.GroupVersion{Group: corev1.GroupName, Version: "v1"}.String(),
					Kind:       "Secret",
					Metadata:   &ObjectMeta{},
				},
			},
		},
		"Unknown": {
			u: &kunstructured.Unstructured{},
			want: want{
				kr: GenericResource{
					Metadata: &ObjectMeta{},
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
