package model

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
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
			Message:            &c.Message,
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
			Message:            &c.Message,
		}
	}
	return out
}

// GetGenericResource from the suppled Kubernetes resource.
func GetGenericResource(u *kunstructured.Unstructured) (GenericResource, error) {
	raw, err := json.Marshal(u)
	if err != nil {
		return GenericResource{}, errors.Wrap(err, "cannot marshal JSON")
	}

	out := GenericResource{
		ID: ReferenceID{
			APIVersion: u.GetAPIVersion(),
			Kind:       u.GetKind(),
			Namespace:  u.GetNamespace(),
			Name:       u.GetName(),
		},
		APIVersion: u.GetAPIVersion(),
		Kind:       u.GetKind(),
		Metadata:   GetObjectMeta(u),
		Raw:        string(raw),
	}

	return out, nil
}

// GetSecret from the suppled Kubernetes Secret
func GetSecret(s *corev1.Secret) (Secret, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return Secret{}, errors.Wrap(err, "cannot marshal JSON")
	}

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
		Raw:        string(raw),
	}

	return out, nil
}

// GetCustomResourceDefinition from the suppled Kubernetes CRD.
func GetCustomResourceDefinition(crd *extv1.CustomResourceDefinition) (CustomResourceDefinition, error) {
	raw, err := json.Marshal(crd)
	if err != nil {
		return CustomResourceDefinition{}, errors.Wrap(err, "cannot marshal JSON")
	}

	out := CustomResourceDefinition{
		ID: ReferenceID{
			APIVersion: crd.APIVersion,
			Kind:       crd.Kind,
			Name:       crd.GetName(),
		},

		APIVersion: crd.APIVersion,
		Kind:       crd.Kind,
		Metadata:   GetObjectMeta(crd),
		Spec: &CustomResourceDefinitionSpec{
			Group: crd.Spec.Group,
			Names: &CustomResourceDefinitionNames{
				Plural:     crd.Spec.Names.Plural,
				Singular:   &crd.Spec.Names.Singular,
				ShortNames: crd.Spec.Names.ShortNames,
				Kind:       crd.Spec.Names.Kind,
				ListKind:   &crd.Spec.Names.ListKind,
				Categories: crd.Spec.Names.Categories,
			},
			Versions: GetCustomResourceDefinitionVersions(crd.Spec.Versions),
		},
		Status: &CustomResourceDefinitionStatus{
			Conditions: GetCustomResourceDefinitionConditions(crd.Status.Conditions),
		},
		Raw: string(raw),
	}

	return out, nil
}

func getIntPtr(i *int64) *int {
	if i == nil {
		return nil
	}

	out := int(*i)
	return &out
}
