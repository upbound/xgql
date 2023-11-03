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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
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
