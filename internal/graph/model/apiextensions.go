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
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/google/go-cmp/cmp"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// A CompositeResourceDefinitionSpec represents the desired state of a
// CompositeResourceDefinition.
type CompositeResourceDefinitionSpec struct {
	Group                string                               `json:"group"`
	Names                *CompositeResourceDefinitionNames    `json:"names"`
	ClaimNames           *CompositeResourceDefinitionNames    `json:"claimNames"`
	ConnectionSecretKeys []string                             `json:"connectionSecretKeys"`
	Versions             []CompositeResourceDefinitionVersion `json:"versions"`

	DefaultCompositionReference  *xpv1.Reference
	EnforcedCompositionReference *xpv1.Reference
}

// GetCompositeResourceDefinitionNames from the supplied Kubernetes names.
func GetCompositeResourceDefinitionNames(in *kextv1.CustomResourceDefinitionNames) *CompositeResourceDefinitionNames {
	if in == nil {
		return nil
	}
	out := &CompositeResourceDefinitionNames{
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

// GetCompositeResourceDefinitionVersions from the supplied Kubernetes versions.
func GetCompositeResourceDefinitionVersions(in []extv1.CompositeResourceDefinitionVersion) []CompositeResourceDefinitionVersion {
	if in == nil {
		return nil
	}

	out := make([]CompositeResourceDefinitionVersion, len(in))
	for i := range in {
		out[i] = CompositeResourceDefinitionVersion{
			Name:          in[i].Name,
			Served:        in[i].Served,
			Referenceable: in[i].Referenceable,
		}

		if s := in[i].Schema; s != nil {
			if raw, err := json.Marshal(s.OpenAPIV3Schema); err == nil {
				out[i].Schema = &CompositeResourceValidation{OpenAPIV3Schema: raw}
			}
		}
	}
	return out
}

// GetCompositeResourceDefinitionControllerStatus from the supplied Crossplane
// controllers
func GetCompositeResourceDefinitionControllerStatus(in extv1.CompositeResourceDefinitionControllerStatus) *CompositeResourceDefinitionControllerStatus {

	out := &CompositeResourceDefinitionControllerStatus{
		CompositeResourceType: &TypeReference{
			APIVersion: in.CompositeResourceTypeRef.APIVersion,
			Kind:       in.CompositeResourceTypeRef.Kind,
		},
		CompositeResourceClaimType: &TypeReference{
			APIVersion: in.CompositeResourceClaimTypeRef.APIVersion,
			Kind:       in.CompositeResourceClaimTypeRef.Kind,
		},
	}

	if cmp.Equal(out, &CompositeResourceDefinitionControllerStatus{CompositeResourceType: &TypeReference{}, CompositeResourceClaimType: &TypeReference{}}) {
		return nil
	}

	return out
}

// GetCompositeResourceDefinitionStatus from the supplied Crossplane status.
func GetCompositeResourceDefinitionStatus(in extv1.CompositeResourceDefinitionStatus) *CompositeResourceDefinitionStatus {
	out := &CompositeResourceDefinitionStatus{
		Conditions:  GetConditions(in.Conditions),
		Controllers: GetCompositeResourceDefinitionControllerStatus(in.Controllers),
	}

	if cmp.Equal(out, &CompositeResourceDefinitionStatus{}) {
		return nil
	}

	return out
}

// GetCompositeResourceDefinition from the supplied Crossplane XRD.
func GetCompositeResourceDefinition(xrd *extv1.CompositeResourceDefinition) CompositeResourceDefinition {
	return CompositeResourceDefinition{
		ID: ReferenceID{
			APIVersion: xrd.APIVersion,
			Kind:       xrd.Kind,
			Name:       xrd.GetName(),
		},
		APIVersion: xrd.APIVersion,
		Kind:       xrd.Kind,
		Metadata:   GetObjectMeta(xrd),
		Spec: &CompositeResourceDefinitionSpec{
			Group:                        xrd.Spec.Group,
			Names:                        GetCompositeResourceDefinitionNames(&xrd.Spec.Names),
			ClaimNames:                   GetCompositeResourceDefinitionNames(xrd.Spec.ClaimNames),
			Versions:                     GetCompositeResourceDefinitionVersions(xrd.Spec.Versions),
			DefaultCompositionReference:  xrd.Spec.DefaultCompositionRef,
			EnforcedCompositionReference: xrd.Spec.EnforcedCompositionRef,
		},
		Status:       GetCompositeResourceDefinitionStatus(xrd.Status),
		Unstructured: unstruct(xrd),
	}
}

// GetCompositionStatus from the supplied Crossplane status.
func GetCompositionStatus(in extv1.CompositionStatus) *CompositionStatus {
	if len(in.Conditions) == 0 {
		return nil
	}
	return &CompositionStatus{Conditions: GetConditions(in.Conditions)}
}

// GetComposition from the supplied Crossplane Composition.
func GetComposition(cmp *extv1.Composition) Composition {
	return Composition{
		ID: ReferenceID{
			APIVersion: cmp.APIVersion,
			Kind:       cmp.Kind,
			Name:       cmp.GetName(),
		},
		APIVersion: cmp.APIVersion,
		Kind:       cmp.Kind,
		Metadata:   GetObjectMeta(cmp),
		Spec: &CompositionSpec{
			CompositeTypeRef: &TypeReference{
				APIVersion: cmp.Spec.CompositeTypeRef.APIVersion,
				Kind:       cmp.Spec.CompositeTypeRef.Kind,
			},
			WriteConnectionSecretsToNamespace: cmp.Spec.WriteConnectionSecretsToNamespace,
		},
		Status:       GetCompositionStatus(cmp.Status),
		Unstructured: unstruct(cmp),
	}
}
