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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// GetEventType from the supplied Kubernetes event type.
func GetEventType(in string) *EventType {
	switch in {
	case "Warning":
		t := EventTypeWarning
		return &t
	case "Normal":
		t := EventTypeNormal
		return &t
	default:
		return nil
	}
}

// GetEvent from the supplied Kubernetes event.
func GetEvent(e *corev1.Event) Event {
	out := Event{
		ID: ReferenceID{
			APIVersion: e.APIVersion,
			Kind:       e.Kind,
			Namespace:  e.GetNamespace(),
			Name:       e.GetName(),
		},
		APIVersion: e.APIVersion,
		Kind:       e.Kind,
		Metadata:   GetObjectMeta(e),
		Type:       GetEventType(e.Type),
		PavedAccess: PavedAccess{
			Paved: paveObject(e),
		},
		InvolvedObjectRef: e.InvolvedObject,
	}

	if e.Reason != "" {
		out.Reason = ptr.To(e.Reason)
	}
	if e.Message != "" {
		out.Message = ptr.To(e.Message)
	}
	if e.Count != 0 {
		c := int(e.Count)
		out.Count = &c
	}
	if e.Source.Component != "" {
		out.Source = &EventSource{Component: ptr.To(e.Source.Component)}
	}
	ft := e.FirstTimestamp.Time
	out.FirstTime = &ft
	lt := e.LastTimestamp.Time
	out.LastTime = &lt

	return out
}
