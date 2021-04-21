package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/test"
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
			want: "O7_pXWsa_kW_6V1r_ktFSyUnJTu_6V1r",
		},
		"ClusterScoped": {
			reason: "It should be possible to encode a cluster scoped ID",
			id: ReferenceID{
				APIVersion: "example.org/v1",
				Kind:       "ExampleKind",
				Name:       "example",
			},
			want: "O7_pXWsa_kW_6V1r_ktFSyY7v-ldaw",
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
			id:     "O7_pXWsa_kW_6V1r_ktFSyUnJTu_6V1r",
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
			id:     "O7_pXWsa_kW_6V1r_ktFSyY7v-ldaw",
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
