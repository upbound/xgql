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
	"github.com/google/go-cmp/cmp/cmpopts"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

func TestGetProviderConfig(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *kunstructured.Unstructured
		want   ProviderConfig
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			u: func() *kunstructured.Unstructured {
				pc := &unstructured.ProviderConfig{Unstructured: kunstructured.Unstructured{}}

				pc.SetAPIVersion("example.org/v1")
				pc.SetKind("ProviderConfig")
				pc.SetName("cool")
				pc.SetConditions(xpv1.Condition{})
				pc.SetUsers(42)

				return pc.GetUnstructured()
			}(),
			want: ProviderConfig{
				ID: ReferenceID{
					APIVersion: "example.org/v1",
					Kind:       "ProviderConfig",
					Name:       "cool",
				},
				APIVersion: "example.org/v1",
				Kind:       "ProviderConfig",
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Status: &ProviderConfigStatus{
					Conditions: []Condition{{}},
					Users:      func() *int { i := 42; return &i }(),
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			u:      &kunstructured.Unstructured{Object: make(map[string]interface{})},
			want: ProviderConfig{
				Metadata: ObjectMeta{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetProviderConfig(tc.u)

			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(ProviderConfig{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetProviderConfig(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
