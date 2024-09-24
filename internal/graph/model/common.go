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
	"io"

	"github.com/99designs/gqlgen/graphql"
	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

// MarshalJSON marshals Unstructured JSON bytes to GraphQL.
func MarshalJSON(val []byte) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = w.Write(val)
	})
}

// UnmarshalJSON marshals Unstructured JSON bytes from GraphQL.
func UnmarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// MarshalStringMap marshals a map[string]string to GraphQL.
func MarshalStringMap(val map[string]string) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_ = json.NewEncoder(w).Encode(val)
	})
}

// UnmarshalStringMap marshals a map[string]string from GraphQL.
func UnmarshalStringMap(v interface{}) (map[string]string, error) {
	if m, ok := v.(map[string]string); ok {
		return m, nil
	}

	return nil, errors.Errorf("%T is not a map", v)
}

// GetConditionStatus from the supplied Crossplane status.
func GetConditionStatus(s corev1.ConditionStatus) ConditionStatus {
	switch s {
	case corev1.ConditionTrue:
		return ConditionStatusTrue
	case corev1.ConditionFalse:
		return ConditionStatusFalse
	case corev1.ConditionUnknown:
		return ConditionStatusUnknown
	default:
		return ConditionStatus(s)
	}
}

// GetConditions from the supplied Crossplane conditions.
func GetConditions(in []xpv1.Condition) []Condition {
	if in == nil {
		return nil
	}

	out := make([]Condition, len(in))
	for i := range in {
		c := in[i] // So we don't take the address of the range variable.

		out[i] = Condition{
			Type:               string(c.Type),
			Status:             GetConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime.Time,
			Reason:             string(c.Reason),
		}
		if c.Message != "" {
			out[i].Message = &c.Message
		}
	}
	return out
}

// GetLabelSelector from the supplied Kubernetes label selector
func GetLabelSelector(s *metav1.LabelSelector) *LabelSelector {
	if s == nil {
		return nil
	}

	return &LabelSelector{MatchLabels: s.MatchLabels}
}

// GetGenericResource from the suppled Kubernetes resource.
func GetGenericResource(u *kunstructured.Unstructured) GenericResource {
	return GenericResource{
		ID: ReferenceID{
			APIVersion: u.GetAPIVersion(),
			Kind:       u.GetKind(),
			Namespace:  u.GetNamespace(),
			Name:       u.GetName(),
		},
		APIVersion: u.GetAPIVersion(),
		Kind:       u.GetKind(),
		Metadata:   GetObjectMeta(u),
		PavedAccess: PavedAccess{
			Paved: fieldpath.Pave(u.Object),
		},
	}
}

// Data of this secret.
func (s Secret) Data(keys []string) map[string]string {
	if keys == nil || s.data == nil {
		return s.data
	}
	out := make(map[string]string)
	for _, k := range keys {
		if v, ok := s.data[k]; ok {
			out[k] = v
		}
	}
	return out
}

// Data of this config map.
func (cm ConfigMap) Data(keys []string) map[string]string {
	if keys == nil || cm.data == nil {
		return cm.data
	}
	out := make(map[string]string)
	for _, k := range keys {
		if v, ok := cm.data[k]; ok {
			out[k] = v
		}
	}
	return out
}

// GetSecret from the suppled Kubernetes Secret
func GetSecret(s *corev1.Secret) Secret {
	out := Secret{
		ID: ReferenceID{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
			Namespace:  s.GetNamespace(),
			Name:       s.GetName(),
		},
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		Metadata:   GetObjectMeta(s),
		PavedAccess: PavedAccess{
			Paved: paveObject(s),
		},
	}

	if s.Data != nil {
		out.data = make(map[string]string)
		for k, v := range s.Data {
			out.data[k] = string(v)
		}
	}

	if s.Type != "" {
		out.Type = ptr.To(string(s.Type))
	}

	return out
}

// GetConfigMap from the supplied Kubernetes ConfigMap.
func GetConfigMap(cm *corev1.ConfigMap) ConfigMap {
	return ConfigMap{
		ID: ReferenceID{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
			Namespace:  cm.GetNamespace(),
			Name:       cm.GetName(),
		},
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "ConfigMap",
		Metadata:   GetObjectMeta(cm),
		PavedAccess: PavedAccess{
			Paved: paveObject(cm),
		},
		data: cm.Data,
	}
}

// GetCustomResourceDefinitionNames from the supplied Kubernetes names.
func GetCustomResourceDefinitionNames(in kextv1.CustomResourceDefinitionNames) *CustomResourceDefinitionNames {
	out := &CustomResourceDefinitionNames{
		Plural:     in.Plural,
		ShortNames: in.ShortNames,
		Kind:       in.Kind,
		Categories: in.Categories,
	}

	if in.Singular != "" {
		out.Singular = &in.Singular
	}
	if in.ListKind != "" {
		out.ListKind = &in.ListKind
	}

	return out
}

// GetResourceScope from the suppled Kubernetes scope.
func GetResourceScope(in kextv1.ResourceScope) ResourceScope {
	switch in {
	case kextv1.ClusterScoped:
		return ResourceScopeClusterScoped
	case kextv1.NamespaceScoped:
		return ResourceScopeNamespaceScoped
	default:
		return ""
	}
}

// GetCustomResourceDefinitionVersions from the supplied Kubernetes versions.
func GetCustomResourceDefinitionVersions(in []kextv1.CustomResourceDefinitionVersion) []CustomResourceDefinitionVersion {
	if in == nil {
		return nil
	}

	out := make([]CustomResourceDefinitionVersion, len(in))
	for i := range in {
		out[i] = CustomResourceDefinitionVersion{
			Name:   in[i].Name,
			Served: in[i].Served,
		}

		if s := in[i].Schema; s != nil && s.OpenAPIV3Schema != nil {
			if raw, err := json.Marshal(s.OpenAPIV3Schema); err == nil {
				out[i].Schema = &CustomResourceValidation{OpenAPIV3Schema: raw}
			}
		}
	}
	return out
}

// GetCustomResourceDefinitionConditions from the supplied Kubernetes CRD
// conditions.
func GetCustomResourceDefinitionConditions(in []kextv1.CustomResourceDefinitionCondition) []Condition {
	if in == nil {
		return nil
	}

	out := make([]Condition, len(in))
	for i := range in {
		c := in[i] // So we don't take the address of the range variable.

		out[i] = Condition{
			Type:               string(c.Type),
			Status:             GetConditionStatus(corev1.ConditionStatus(c.Status)),
			LastTransitionTime: c.LastTransitionTime.Time,
			Reason:             c.Reason,
		}
		if c.Message != "" {
			out[i].Message = &c.Message
		}
	}
	return out
}

// GetCustomResourceDefinitionStatus from the supplied Crossplane status.
func GetCustomResourceDefinitionStatus(in kextv1.CustomResourceDefinitionStatus) *CustomResourceDefinitionStatus {
	if len(in.Conditions) == 0 {
		return nil
	}
	return &CustomResourceDefinitionStatus{Conditions: GetCustomResourceDefinitionConditions(in.Conditions)}
}

// GetCustomResourceDefinition from the suppled Kubernetes CRD.
func GetCustomResourceDefinition(crd *unstructured.CustomResourceDefinition) CustomResourceDefinition {
	return CustomResourceDefinition{
		ID: ReferenceID{
			APIVersion: crd.GetAPIVersion(),
			Kind:       crd.GetKind(),
			Name:       crd.GetName(),
		},

		APIVersion: crd.GetAPIVersion(),
		Kind:       crd.GetKind(),
		Metadata:   GetObjectMeta(crd),
		Spec: CustomResourceDefinitionSpec{
			Group:    crd.GetSpecGroup(),
			Names:    *GetCustomResourceDefinitionNames(crd.GetSpecNames()),
			Scope:    GetResourceScope(crd.GetSpecScope()),
			Versions: GetCustomResourceDefinitionVersions(crd.GetSpecVersions()),
		},
		Status: GetCustomResourceDefinitionStatus(crd.GetStatus()),
		PavedAccess: PavedAccess{
			Paved: fieldpath.Pave(crd.Object),
		},
	}
}

// GetKubernetesResource from the supplied unstructured Kubernetes resource.
// GetKubernetesResource attempts to determine what type of resource the
// unstructured data contains (e.g. a managed resource, a provider, etc) and
// return the appropriate model type. If no type can be detected it returns a
// GenericResource.
func GetKubernetesResource(u *kunstructured.Unstructured) (KubernetesResource, error) { //nolint:gocyclo
	// This isn't _really_ that complex; it's a long but simple switch.

	switch {

	case unstructured.ProbablyProviderConfig(u):
		return GetProviderConfig(u), nil

	case unstructured.ProbablyComposite(u):
		return GetCompositeResource(u), nil

	case unstructured.ProbablyClaim(u):
		return GetCompositeResourceClaim(u), nil

	// Note that order is important here. We want to check whether the resource
	// seems to be a managed resource _after_ checking whether it seems to be a
	// composite resource because it's possible to define a composite resource
	// that would pass the ProbablyManaged check. Such a composite resource
	// would very likely pass the ProbablyComposite check and never reach this.
	case unstructured.ProbablyManaged(u):
		return GetManagedResource(u), nil

	case u.GroupVersionKind() == pkgv1.ProviderGroupVersionKind:
		p := &pkgv1.Provider{}
		if err := convert(u, p); err != nil {
			return nil, errors.Wrap(err, "cannot convert provider")
		}
		return GetProvider(p), nil

	case u.GroupVersionKind() == pkgv1.ProviderRevisionGroupVersionKind:
		pr := &pkgv1.ProviderRevision{}
		if err := convert(u, pr); err != nil {
			return nil, errors.Wrap(err, "cannot convert provider revision")
		}
		return GetProviderRevision(pr), nil

	case u.GroupVersionKind() == pkgv1.ConfigurationGroupVersionKind:
		c := &pkgv1.Configuration{}
		if err := convert(u, c); err != nil {
			return nil, errors.Wrap(err, "cannot convert configuration")
		}
		return GetConfiguration(c), nil

	case u.GroupVersionKind() == pkgv1.ConfigurationRevisionGroupVersionKind:
		cr := &pkgv1.ConfigurationRevision{}
		if err := convert(u, cr); err != nil {
			return nil, errors.Wrap(err, "cannot convert configuration revision")
		}
		return GetConfigurationRevision(cr), nil

	case u.GroupVersionKind() == extv1.CompositeResourceDefinitionGroupVersionKind:
		xrd := &extv1.CompositeResourceDefinition{}
		if err := convert(u, xrd); err != nil {
			return nil, errors.Wrap(err, "cannot convert composite resource definition")
		}
		return GetCompositeResourceDefinition(xrd), nil

	case u.GroupVersionKind() == extv1.CompositionGroupVersionKind:
		cmp := &extv1.Composition{}
		if err := convert(u, cmp); err != nil {
			return nil, errors.Wrap(err, "cannot convert composition")
		}
		return GetComposition(cmp), nil

	case u.GroupVersionKind() == schema.GroupVersionKind{Group: kextv1.GroupName, Version: "v1", Kind: "CustomResourceDefinition"}:
		crd := &unstructured.CustomResourceDefinition{}
		crd.SetAPIVersion("apiextensions.k8s.io/v1")
		crd.SetKind("CustomResourceDefinition")
		if err := convert(u, crd); err != nil {
			return nil, errors.Wrap(err, "cannot convert custom resource definition")
		}
		return GetCustomResourceDefinition(crd), nil

	case u.GroupVersionKind() == schema.GroupVersionKind{Group: corev1.GroupName, Version: "v1", Kind: "Secret"}:
		sec := &corev1.Secret{}
		if err := convert(u, sec); err != nil {
			return nil, errors.Wrap(err, "cannot convert secret")
		}
		return GetSecret(sec), nil

	case u.GroupVersionKind() == schema.GroupVersionKind{Group: corev1.GroupName, Version: "v1", Kind: "ConfigMap"}:
		cm := &corev1.ConfigMap{}
		if err := convert(u, cm); err != nil {
			return nil, errors.Wrap(err, "cannot convert config map")
		}
		return GetConfigMap(cm), nil

	default:
		return GetGenericResource(u), nil
	}
}

// GetObjectReference from the supplied corev1 ObjectReference
func GetObjectReference(o *corev1.ObjectReference) *ObjectReference {

	if o == nil {
		return nil
	}

	ref := &ObjectReference{}

	if o.Kind != "" {
		ref.Kind = &o.Kind
	}

	if o.Namespace != "" {
		ref.Namespace = &o.Namespace
	}

	if o.Name != "" {
		ref.Name = &o.Name
	}

	return ref
}

// GetSecretReference from the supplied corev1 ObjectReference
func GetSecretReference(u *xpv1.SecretReference) *SecretReference {
	if u == nil {
		return nil
	}

	return &SecretReference{
		Name:      u.Name,
		Namespace: u.Namespace,
	}
}
