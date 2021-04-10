package model

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

// Reference ID separator.
const sep = "|"

// Reference ID encoder.
var encoder = base64.RawStdEncoding

// Reference ID parsing errors.
var (
	errDecode    = "cannot decode id"
	errMalformed = "malformed id"
	errParse     = "cannot parse id"
	errType      = "id must be a string"
)

// A ReferenceID uniquely represents a Kubernetes resource in GraphQL. It
// encodes to a String per the documentation of its String method, but is
// otherwise similar to the 'Reference' types (e.g. corev1.ObjectReference) that
// are used to identify Kubernetes objects.
type ReferenceID struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// ParseReferenceID parses the supplied ID string.
func ParseReferenceID(id string) (ReferenceID, error) {
	b, err := encoder.DecodeString(id)
	if err != nil {
		return ReferenceID{}, errors.Wrap(err, errDecode)
	}

	parts := strings.Split(string(b), sep)
	if len(parts) != 4 {
		return ReferenceID{}, errors.New(errMalformed)
	}

	out := ReferenceID{
		APIVersion: parts[0],
		Kind:       parts[1],
		Namespace:  parts[2],
		Name:       parts[3],
	}

	return out, nil
}

// A String representation of a ReferenceID. The idea is to store the data that
// uniquely identifies a resource in the Kubernetes API (a reference) such that
// we can extract that data from a given ID string in future. Representing this
// data as a string gives GraphQL clients a single, idiomatic scalar field they
// may consider the "primary key" of a resource.
//
// We serialise the reference as "apiVersion|kind|namespace|name", then base64
// encode it in a mild attempt to reinforce the fact that clients must treat it
// as opaque. Cluster scoped resources have an empty namespace 'field', i.e.
// "apiVersion|kind||name"
func (id *ReferenceID) String() string {
	return encoder.EncodeToString([]byte(id.APIVersion + sep + id.Kind + sep + id.Namespace + sep + id.Name))
}

// UnmarshalGQL unmarshals a ReferenceID.
func (id *ReferenceID) UnmarshalGQL(v interface{}) error {
	s, ok := v.(string)
	if !ok {
		return errors.New(errType)
	}
	in, err := ParseReferenceID(s)
	if err != nil {
		return errors.Wrap(err, errParse)
	}

	*id = in
	return nil
}

// MarshalGQL marshals a ReferenceID as a string.
func (id ReferenceID) MarshalGQL(w io.Writer) {
	_, _ = w.Write([]byte(`"` + id.String() + `"`))
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
			Status:             ConditionStatus(c.Status),
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

	ml := map[string]interface{}{}
	for k, v := range s.MatchLabels {
		ml[k] = v
	}

	return &LabelSelector{MatchLabels: ml}
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
		Raw:        raw(u),
	}
}

// A Secret holds secret data.
type Secret struct {
	// An opaque identifier that is unique across all types.
	ID ReferenceID `json:"id"`

	// The underlying Kubernetes API version of this resource.
	APIVersion string `json:"apiVersion"`

	// The underlying Kubernetes API kind of this resource.
	Kind string `json:"kind"`

	// Metadata that is common to all Kubernetes API resources.
	Metadata *ObjectMeta `json:"metadata"`

	// Type of this secret.
	Type *string `json:"type"`

	// A raw JSON representation of the underlying Kubernetes resource.
	Raw string `json:"raw"`

	Data map[string][]byte
}

// IsNode indicates that a Secret satisfies the GraphQL Node interface.
func (Secret) IsNode() {}

// IsKubernetesResource indicates that a Secret satisfies the GraphQL
// IsKubernetesResource interface.
func (Secret) IsKubernetesResource() {}

// GetSecret from the suppled Kubernetes Secret
func GetSecret(s *corev1.Secret) Secret {
	out := Secret{
		ID: ReferenceID{
			APIVersion: s.APIVersion,
			Kind:       s.Kind,
			Namespace:  s.GetNamespace(),
			Name:       s.GetName(),
		},

		APIVersion: s.APIVersion,
		Kind:       s.Kind,
		Metadata:   GetObjectMeta(s),
		Data:       s.Data,
		Raw:        raw(s),
	}

	if s.Type != "" {
		out.Type = pointer.StringPtr(string(s.Type))
	}

	return out
}

// A ConfigMap holds configuration data.
type ConfigMap struct {
	// An opaque identifier that is unique across all types.
	ID ReferenceID `json:"id"`

	// The underlying Kubernetes API version of this resource.
	APIVersion string `json:"apiVersion"`

	// The underlying Kubernetes API kind of this resource.
	Kind string `json:"kind"`

	// Metadata that is common to all Kubernetes API resources.
	Metadata *ObjectMeta `json:"metadata"`

	// A raw JSON representation of the underlying Kubernetes resource.
	Raw string `json:"raw"`

	Data map[string]string
}

// IsNode indicates that a ConfigMap satisfies the GraphQL Node interface.
func (ConfigMap) IsNode() {}

// IsKubernetesResource indicates that a ConfigMap satisfies the GraphQL
// IsKubernetesResource interface.
func (ConfigMap) IsKubernetesResource() {}

// GetConfigMap from the supplied Kubernetes ConfigMap.
func GetConfigMap(cm *corev1.ConfigMap) ConfigMap {
	return ConfigMap{
		ID: ReferenceID{
			APIVersion: cm.APIVersion,
			Kind:       cm.Kind,
			Namespace:  cm.GetNamespace(),
			Name:       cm.GetName(),
		},

		APIVersion: cm.APIVersion,
		Kind:       cm.Kind,
		Metadata:   GetObjectMeta(cm),
		Data:       cm.Data,
		Raw:        raw(cm),
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
				schema := string(raw)
				out[i].Schema = &CustomResourceValidation{OpenAPIV3Schema: &schema}
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
			Status:             ConditionStatus(c.Status),
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
func GetCustomResourceDefinition(crd *kextv1.CustomResourceDefinition) CustomResourceDefinition {
	return CustomResourceDefinition{
		ID: ReferenceID{
			APIVersion: crd.APIVersion,
			Kind:       crd.Kind,
			Name:       crd.GetName(),
		},

		APIVersion: crd.APIVersion,
		Kind:       crd.Kind,
		Metadata:   GetObjectMeta(crd),
		Spec: &CustomResourceDefinitionSpec{
			Group:    crd.Spec.Group,
			Names:    GetCustomResourceDefinitionNames(crd.Spec.Names),
			Versions: GetCustomResourceDefinitionVersions(crd.Spec.Versions),
		},
		Status: GetCustomResourceDefinitionStatus(crd.Status),
		Raw:    raw(crd),
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
	case unstructured.ProbablyManaged(u):
		return GetManagedResource(u), nil

	case unstructured.ProbablyProviderConfig(u):
		return GetProviderConfig(u), nil

	case unstructured.ProbablyComposite(u):
		return GetCompositeResource(u), nil

	case unstructured.ProbablyClaim(u):
		return GetCompositeResourceClaim(u), nil

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
		crd := &kextv1.CustomResourceDefinition{}
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

	default:
		return GetGenericResource(u), nil
	}
}
