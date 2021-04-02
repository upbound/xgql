package model

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GetConnectionDetailsLastPublishedTime from the supplied Kubernetes time.
func GetConnectionDetailsLastPublishedTime(t *metav1.Time) *time.Time {
	if t == nil {
		return nil
	}
	return &t.Time
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
		Status: &CompositeResourceStatus{
			Conditions: GetConditions(xr.GetConditions()),
			ConnectionDetails: &CompositeResourceConnectionDetails{
				LastPublishedTime: GetConnectionDetailsLastPublishedTime(xr.GetConnectionDetailsLastPublishedTime()),
			},
		},
		Raw: raw(xr),
	}
}
