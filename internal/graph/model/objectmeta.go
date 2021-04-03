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
	om := &ObjectMeta{
		Name:            m.GetName(),
		UID:             string(m.GetUID()),
		ResourceVersion: m.GetResourceVersion(),
		Generation:      int(m.GetGeneration()),
		CreationTime:    m.GetCreationTimestamp().Time,
		OwnerReferences: m.GetOwnerReferences(),
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
	if in := m.GetLabels(); len(in) > 0 {
		om.Labels = map[string]interface{}{}
		for k, v := range in {
			om.Labels[k] = v
		}
	}
	if in := m.GetAnnotations(); len(in) > 0 {
		om.Annotations = map[string]interface{}{}
		for k, v := range in {
			om.Annotations[k] = v
		}
	}

	return om
}
