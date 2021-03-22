package model

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
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
