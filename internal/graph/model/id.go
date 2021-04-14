package model

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// Reference ID separator.
const sep = "|"

// Reference ID encoder.
var encoder = base64.RawURLEncoding

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
