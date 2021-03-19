package model

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// ObjectMeta that is common to all Kubernetes objects.
type ObjectMeta struct {
	Name            string                 `json:"name"`
	GenerateName    *string                `json:"generateName"`
	Namespace       *string                `json:"namespace"`
	UID             string                 `json:"uid"`
	ResourceVersion string                 `json:"resourceVersion"`
	Generation      int                    `json:"generation"`
	CreationTime    time.Time              `json:"creationTime"`
	DeletionTime    *time.Time             `json:"deletionTime"`
	Labels          map[string]interface{} `json:"labels"`
	Annotations     map[string]interface{} `json:"annotations"`

	OwnerReferences []metav1.OwnerReference
}

// GetObjectMeta from the supplied Kubernetes object.
func GetObjectMeta(m metav1.Object) *ObjectMeta {
	l := map[string]interface{}{}
	for k, v := range m.GetLabels() {
		l[k] = v
	}

	a := map[string]interface{}{}
	for k, v := range m.GetAnnotations() {
		a[k] = v
	}

	var dt *time.Time = nil
	if t := m.GetDeletionTimestamp(); t != nil {
		dt = &t.Time
	}

	return &ObjectMeta{
		Name:            m.GetName(),
		GenerateName:    pointer.StringPtr(m.GetGenerateName()),
		Namespace:       pointer.StringPtr(m.GetNamespace()),
		UID:             string(m.GetUID()),
		ResourceVersion: m.GetResourceVersion(),
		Generation:      int(m.GetGeneration()),
		CreationTime:    m.GetCreationTimestamp().Time,
		DeletionTime:    dt,
		Labels:          l,
		Annotations:     a,
		OwnerReferences: m.GetOwnerReferences(),
	}
}
