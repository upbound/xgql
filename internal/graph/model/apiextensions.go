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
