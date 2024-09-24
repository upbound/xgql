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

	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/google/go-cmp/cmp"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// A CompositeResourceDefinitionSpec represents the desired state of a
// CompositeResourceDefinition.
type CompositeResourceDefinitionSpec struct {
	Group                string                               `json:"group"`
	Names                CompositeResourceDefinitionNames     `json:"names"`
	ClaimNames           *CompositeResourceDefinitionNames    `json:"claimNames"`
	ConnectionSecretKeys []string                             `json:"connectionSecretKeys"`
	Versions             []CompositeResourceDefinitionVersion `json:"versions"`

	DefaultCompositionReference  *extv1.CompositionReference
	EnforcedCompositionReference *extv1.CompositionReference
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
		Spec: CompositeResourceDefinitionSpec{
			Group:                        xrd.Spec.Group,
			Names:                        *GetCompositeResourceDefinitionNames(&xrd.Spec.Names),
			ClaimNames:                   GetCompositeResourceDefinitionNames(xrd.Spec.ClaimNames),
			Versions:                     GetCompositeResourceDefinitionVersions(xrd.Spec.Versions),
			DefaultCompositionReference:  xrd.Spec.DefaultCompositionRef,
			EnforcedCompositionReference: xrd.Spec.EnforcedCompositionRef,
		},
		Status: GetCompositeResourceDefinitionStatus(xrd.Status),
		PavedAccess: PavedAccess{
			Paved: paveObject(xrd),
		},
	}
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
		Spec: CompositionSpec{
			CompositeTypeRef: TypeReference{
				APIVersion: cmp.Spec.CompositeTypeRef.APIVersion,
				Kind:       cmp.Spec.CompositeTypeRef.Kind,
			},
			WriteConnectionSecretsToNamespace: cmp.Spec.WriteConnectionSecretsToNamespace,
		},
		PavedAccess: PavedAccess{
			Paved: paveObject(cmp),
		},
	}
}

/* Handle deprecated items preferring non-deprecated */
func (options *DefinedCompositeResourceOptionsInput) DeprecationPatch(version *string) {
	if version != nil && options.Version == nil {
		options.Version = version
	}
}

/* Handle deprecated items preferring non-deprecated */
func (options *DefinedCompositeResourceClaimOptionsInput) DeprecationPatch(version *string, namespace *string) {
	if version != nil && options.Version == nil {
		options.Version = version
	}
	if namespace != nil && options.Namespace == nil {
		options.Namespace = namespace
	}
}

/* A model that has conditions */
type ConditionedModel interface {
	GetConditions() []Condition
}

func (m *CompositeResourceClaim) GetConditions() []Condition {
	if m.Status != nil {
		return m.Status.Conditions
	}
	return nil
}

func (m *CompositeResource) GetConditions() []Condition {
	if m.Status != nil {
		return m.Status.Conditions
	}
	return nil
}
