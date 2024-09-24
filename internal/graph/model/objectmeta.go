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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ObjectMeta that is common to all Kubernetes objects.
type ObjectMeta struct {
	Name            string     `json:"name"`
	GenerateName    *string    `json:"generateName"`
	Namespace       *string    `json:"namespace"`
	UID             string     `json:"uid"`
	ResourceVersion string     `json:"resourceVersion"`
	Generation      int        `json:"generation"`
	CreationTime    time.Time  `json:"creationTime"`
	DeletionTime    *time.Time `json:"deletionTime"`

	OwnerReferences []metav1.OwnerReference
	labels          map[string]string
	annotations     map[string]string
}

// GetObjectMeta from the supplied Kubernetes object.
func GetObjectMeta(m metav1.Object) ObjectMeta {
	om := &ObjectMeta{
		Name:            m.GetName(),
		UID:             string(m.GetUID()),
		ResourceVersion: m.GetResourceVersion(),
		Generation:      int(m.GetGeneration()),
		CreationTime:    m.GetCreationTimestamp().Time,
		OwnerReferences: m.GetOwnerReferences(),
		labels:          m.GetLabels(),
		annotations:     m.GetAnnotations(),
	}

	if n := m.GetGenerateName(); n != "" {
		om.GenerateName = ptr.To(n)
	}
	if n := m.GetNamespace(); n != "" {
		om.Namespace = ptr.To(n)
	}
	if t := m.GetDeletionTimestamp(); t != nil {
		om.DeletionTime = &t.Time
	}

	return *om
}

// Labels this ObjectMeta contains.
func (om ObjectMeta) Labels(keys []string) map[string]string {
	if keys == nil || om.labels == nil {
		return om.labels
	}
	out := make(map[string]string)
	for _, k := range keys {
		if v, ok := om.labels[k]; ok {
			out[k] = v
		}
	}
	return out
}

// Annotations this ObjectMeta contains.
func (om ObjectMeta) Annotations(keys []string) map[string]string {
	if keys == nil || om.annotations == nil {
		return om.annotations
	}
	out := make(map[string]string)
	for _, k := range keys {
		if v, ok := om.annotations[k]; ok {
			out[k] = v
		}
	}
	return out
}
