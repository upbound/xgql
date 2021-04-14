package model

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/kjk/smaz"
	"github.com/pkg/errors"
)

// Reference ID separator.
const sep = "|"

// Reference ID encoder.
var encoder = base64.RawURLEncoding

// Reference ID parsing errors.
var (
	errDecode     = "cannot decode id"
	errDecompress = "cannot decompress id"
	errMalformed  = "malformed id"
	errParse      = "cannot parse id"
	errType       = "id must be a string"
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
	s, err := encoder.DecodeString(id)
	if err != nil {
		return ReferenceID{}, errors.Wrap(err, errDecode)
	}

	b, err := smaz.Decode(nil, s)
	if err != nil {
		return ReferenceID{}, errors.Wrap(err, errDecompress)
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
// We serialise the reference as "apiVersion|kind|namespace|name", then compress
// and base64 encode it. This encourages consumers to treat IDs as opaque data,
// and makes them relatively URL-friendly. Cluster scoped resources have an
// empty namespace 'field', i.e. "apiVersion|kind||name"
func (id *ReferenceID) String() string {
	s := smaz.Encode(nil, []byte(id.APIVersion+sep+id.Kind+sep+id.Namespace+sep+id.Name))
	return encoder.EncodeToString(s)
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

func init() {
	// NOTE(negz): This table cannot be longer than 254 strings. Updating the
	// table is a breaking change; xgql can only understand IDs compressed using
	// an identical table.
	smaz.LoadCustomTable([]string{
		// Things that are likely to appear in xgql IDs.
		"apiextensions.crossplane.io/v1|CompositeResourceDefinition||",
		"apiextensions.crossplane.io/v1|Composition||",
		"pkg.crossplane.io/v1|Configuration||",
		"pkg.crossplane.io/v1|ConfigurationRevision||",
		"pkg.crossplane.io/v1|Package||",
		"pkg.crossplane.io/v1|PackageRevision||",
		"apiextensions.k8s.io/v1|CustomResourceDefinition||",
		".crossplane.io/v1|", ".crossplane.io/v1alpha1|", ".crossplane.io/v1beta1|", ".crossplane.io/v",
		"crossplane", ".crossplane.io",
		".k8s.io/v", ".k8s.io/v1|", ".k8s.io/v1beta1|",
		"/v1|", "/v1alpha1|", "/v1beta1|", "/v1beta", "/v1alpha", "/v2|", "/v", "alpha", "beta",
		".io/v1|", ".io/v1alpha1|", ".io/v1beta1|",
		".dev/v1|", ".io/v1alpha1|", ".dev/v1beta1|",
		".com/v1|", ".com/v1alpha1|", ".com/v1beta1|",
		".net/v1|", ".net/v1alpha1|", ".net/v1beta1|",
		".org/v1|", ".org/v1alpha1|", ".org/v1beta1|",
		"|ProviderConfig||", "ProviderConfig||default",
		"|", "||",
		"default", "prod", "dev", "staging",
		"Composite", "Cluster", "Cloud", "Instance", "Claim", "XR",
		"composite", "cluster", "cloud", "instance", "claim", "xr",
		".k8s.io", ".com", ".net", ".org", ".io", ".dev",

		// Things from smaz'z generic table that may appear in an ID.
		"the", "e", "t", "a", "of", "o", "and", "i", "n", "s", "r", "in", "he",
		"th", "h", "to", "l", "d", "an", "er", "c", "on", "re", "is", "u", "at",
		"or", "which", "f", "m", "as", "it", "that", "was", "en", "es", "g",
		"p", "nd", "w", "ed", "for", "te", "ing", "ti", "his", "st", "ar", "nt",
		"y", "ng", "with", "le", "al", "b", "ou", "be", "were", "se", "ent",
		"ha", "their", "hi", "from", "de", "ion", "me", "v", ".", "ve", "all",
		"ri", "ro", "co", "are", "ea", "her", "by", "they", "di", "ra", "ic",
		"not", "ce", "la", "ne", "tio", "io", "we", "om", "ur", "li", "ll",
		"ch", "had", "this", "ere", "us", "ss", "ma", "one", "but", "el", "so",
		"no", "ter", "iv", "ho", "hat", "ns", "wh", "tr", "ut", "have", "ta",
		"tha", "-", "ati", "pe", "there", "ass", "si", "wa", "ec", "our", "who",
		"its", "z", "fo", "rs", "ot", "un", "im", "nc", "ate", "ver", "ad",
		"ly", "ee", "id", "ac", "il", "rt", "whi", "ge", "x", "men"})
}
