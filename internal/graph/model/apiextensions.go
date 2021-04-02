package model

import (
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"

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

	DefaultCompositionRef  *xpv1.Reference
	EnforcedCompositionRef *xpv1.Reference
}

// GetCompositeResourceDefinitionClaimNames from the supplied Crossplane
// versions.
func GetCompositeResourceDefinitionClaimNames(in *kextv1.CustomResourceDefinitionNames) *CompositeResourceDefinitionNames {
	if in == nil {
		return nil
	}
	return &CompositeResourceDefinitionNames{
		Plural:     in.Plural,
		Singular:   &in.Singular,
		ShortNames: in.ShortNames,
		Kind:       in.Kind,
		ListKind:   &in.ListKind,
		Categories: in.Categories,
	}
}

// GetCompositeResourceDefinitionVersions from the supplied Kubernetes versions.
func GetCompositeResourceDefinitionVersions(in []extv1.CompositeResourceDefinitionVersion) []CompositeResourceDefinitionVersion {
	if in == nil {
		return nil
	}

	out := make([]CompositeResourceDefinitionVersion, len(in))
	for i := range in {
		out[i] = CompositeResourceDefinitionVersion{
			Name:   in[i].Name,
			Served: in[i].Served,
		}

		if s := in[i].Schema; s != nil {
			if raw, err := json.Marshal(s.OpenAPIV3Schema); err == nil {
				schema := string(raw)
				out[i].Schema = &CompositeResourceValidation{OpenAPIV3Schema: &schema}
			}
		}
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
			Group: xrd.Spec.Group,
			Names: &CompositeResourceDefinitionNames{
				Plural:     xrd.Spec.Names.Plural,
				Singular:   &xrd.Spec.Names.Singular,
				ShortNames: xrd.Spec.Names.ShortNames,
				Kind:       xrd.Spec.Names.Kind,
				ListKind:   &xrd.Spec.Names.ListKind,
				Categories: xrd.Spec.Names.Categories,
			},
			ClaimNames:             GetCompositeResourceDefinitionClaimNames(xrd.Spec.ClaimNames),
			Versions:               GetCompositeResourceDefinitionVersions(xrd.Spec.Versions),
			DefaultCompositionRef:  xrd.Spec.DefaultCompositionRef,
			EnforcedCompositionRef: xrd.Spec.EnforcedCompositionRef,
		},
		Status: &CompositeResourceDefinitionStatus{
			Conditions: GetConditions(xrd.Status.Conditions),
		},
		Raw: raw(xrd),
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
		Spec: &CompositionSpec{
			CompositeTypeRef: &TypeReference{
				APIVersion: cmp.Spec.CompositeTypeRef.APIVersion,
				Kind:       cmp.Spec.CompositeTypeRef.Kind,
			},
			WriteConnectionSecretsToNamespace: cmp.Spec.WriteConnectionSecretsToNamespace,
		},
		Status: &CompositionStatus{
			Conditions: GetConditions(cmp.Status.Conditions),
		},
		Raw: raw(cmp),
	}
}
