package model

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// ObjectMeta that is common to all Kubernetes objects.
type ObjectMeta struct {
	Name            string            `json:"name"`
	GenerateName    *string           `json:"generateName"`
	Namespace       *string           `json:"namespace"`
	UID             string            `json:"uid"`
	ResourceVersion string            `json:"resourceVersion"`
	Generation      int               `json:"generation"`
	CreationTime    time.Time         `json:"creationTime"`
	DeletionTime    *time.Time        `json:"deletionTime"`
	Labels          map[string]string `json:"labels"`
	Annotations     map[string]string `json:"annotations"`

	OwnerReferences []metav1.OwnerReference
}

// GetObjectMeta from the supplied Kubernetes object.
func GetObjectMeta(m metav1.Object) *ObjectMeta {
	om := &ObjectMeta{
		Name:            m.GetName(),
		UID:             string(m.GetUID()),
		ResourceVersion: m.GetResourceVersion(),
		Generation:      int(m.GetGeneration()),
		CreationTime:    m.GetCreationTimestamp().Time,
		OwnerReferences: m.GetOwnerReferences(),
		Labels:          m.GetLabels(),
		Annotations:     m.GetAnnotations(),
	}

	if n := m.GetGenerateName(); n != "" {
		om.GenerateName = pointer.StringPtr(n)
	}
	if n := m.GetNamespace(); n != "" {
		om.Namespace = pointer.StringPtr(n)
	}
	if t := m.GetDeletionTimestamp(); t != nil {
		om.DeletionTime = &t.Time
	}

	return om
}
