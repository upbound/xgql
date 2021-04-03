package model

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/upbound/xgql/internal/unstructured"
)

// A CompositeResourceSpec defines the desired state of a composite resource.
type CompositeResourceSpec struct {
	CompositionSelector *LabelSelector `json:"compositionSelector"`

	CompositionReference              *corev1.ObjectReference
	ClaimReference                    *corev1.ObjectReference
	ResourceReferences                []corev1.ObjectReference
	WritesConnectionSecretToReference *xpv1.SecretReference
}

// GetCompositeResourceStatus from the supplied Crossplane composite.
func GetCompositeResourceStatus(xr *unstructured.Composite) *CompositeResourceStatus {
	c := xr.GetConditions()
	t := xr.GetConnectionDetailsLastPublishedTime()

	out := &CompositeResourceStatus{}
	if len(c) > 0 {
		out.Conditions = GetConditions(c)
	}
	if t != nil {
		out.ConnectionDetails = &CompositeResourceConnectionDetails{LastPublishedTime: &t.Time}
	}

	if cmp.Equal(out, &CompositeResourceStatus{}) {
		return nil
	}

	return out
}

// GetCompositeResource from the supplied Crossplane resource.
func GetCompositeResource(u *kunstructured.Unstructured) CompositeResource {
	xr := &unstructured.Composite{Unstructured: *u}
	return CompositeResource{
		ID: ReferenceID{
			APIVersion: xr.GetAPIVersion(),
			Kind:       xr.GetKind(),
			Name:       xr.GetName(),
		},

		APIVersion: xr.GetAPIVersion(),
		Kind:       xr.GetKind(),
		Metadata:   GetObjectMeta(xr),
		Spec: &CompositeResourceSpec{
			CompositionSelector:               GetLabelSelector(xr.GetCompositionSelector()),
			CompositionReference:              xr.GetCompositionReference(),
			ClaimReference:                    xr.GetClaimReference(),
			ResourceReferences:                xr.GetResourceReferences(),
			WritesConnectionSecretToReference: xr.GetWriteConnectionSecretToReference(),
		},
		Status: GetCompositeResourceStatus(xr),
		Raw:    raw(xr),
	}
}
