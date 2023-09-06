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
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/xgql/internal/unstructured"
)

// GetProviderConfigStatus from the supplied Crossplane status.
func GetProviderConfigStatus(pc *unstructured.ProviderConfig) *ProviderConfigStatus {
	users := int(pc.GetUsers())
	out := &ProviderConfigStatus{
		Conditions: GetConditions(pc.GetConditions()),
	}
	if users > 0 {
		out.Users = &users
	}
	if cmp.Equal(out, &ProviderConfigStatus{}) {
		return nil
	}
	return out
}

// GetProviderConfig from the suppled Crossplane ProviderConfig.
func GetProviderConfig(u *kunstructured.Unstructured) ProviderConfig {
	pc := &unstructured.ProviderConfig{Unstructured: *u}
	return ProviderConfig{
		ID: ReferenceID{
			APIVersion: pc.GetAPIVersion(),
			Kind:       pc.GetKind(),
			Name:       pc.GetName(),
		},

		APIVersion: pc.GetAPIVersion(),
		Kind:       pc.GetKind(),
		Metadata:   GetObjectMeta(pc),
		Status:     GetProviderConfigStatus(pc),
		PavedAccess: PavedAccess{
			Paved: fieldpath.Pave(u.Object),
		},
	}
}
