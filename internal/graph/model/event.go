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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

// An Event pertaining to a Kubernetes resource.
type Event struct {
	// An opaque identifier that is unique across all types.
	ID ReferenceID `json:"id"`

	// The underlying Kubernetes API version of this resource.
	APIVersion string `json:"apiVersion"`

	// The underlying Kubernetes API kind of this resource.
	Kind string `json:"kind"`

	// Metadata that is common to all Kubernetes API resources.
	Metadata *ObjectMeta `json:"metadata"`

	// The type of event.
	Type *EventType `json:"type"`

	// The reason the event was emitted.
	Reason *string `json:"reason"`

	// Details about the event, if any.
	Message *string `json:"message"`

	// The source of the event - e.g. a controller.
	Source *EventSource `json:"source"`

	// The number of times this event has occurred.
	Count *int `json:"count"`

	// The time at which this event was first recorded.
	FirstTime *time.Time `json:"firstTime"`

	// The time at which this event was most recently recorded.
	LastTime *time.Time `json:"lastTime"`

	// An unstructured JSON representation of the event.
	Unstructured []byte `json:"raw"`

	InvolvedObjectRef corev1.ObjectReference
}

// IsNode indicates that an Event satisfies the GraphQL node interface.
func (Event) IsNode() {}

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
		APIVersion:        e.APIVersion,
		Kind:              e.Kind,
		Metadata:          GetObjectMeta(e),
		Type:              GetEventType(e.Type),
		Unstructured:      unstruct(e),
		InvolvedObjectRef: e.InvolvedObject,
	}

	if e.Reason != "" {
		out.Reason = pointer.String(e.Reason)
	}
	if e.Message != "" {
		out.Message = pointer.String(e.Message)
	}
	if e.Count != 0 {
		c := int(e.Count)
		out.Count = &c
	}
	if e.Source.Component != "" {
		out.Source = &EventSource{Component: pointer.String(e.Source.Component)}
	}
	ft := e.FirstTimestamp.Time
	out.FirstTime = &ft
	lt := e.LastTimestamp.Time
	out.LastTime = &lt

	return out
}
