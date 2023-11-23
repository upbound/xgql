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

package resolvers

import (
	"context"
	"sort"

	"github.com/99designs/gqlgen/graphql"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errListEvents    = "cannot list events"
	errGetInvolved   = "cannot get involved resource"
	errModelInvolved = "cannot model involved resource"
)

type events struct {
	clients ClientCache
}

func (r *events) Resolve(ctx context.Context, obj *corev1.ObjectReference) (model.EventConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.EventConnection{}, nil
	}

	in := &corev1.EventList{}
	if err := c.List(ctx, in, client.UnsafeDisableDeepCopyOption(true)); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListEvents))
		return model.EventConnection{}, nil
	}

	// If no involved object was supplied we want to fetch all events. This may
	// include Kubernetes events that don't pertain to Crossplane.
	if obj == nil {
		ordered := timeOrderedEventIndices{
			indices: make([]int, len(in.Items)),
			items:   in.Items,
		}
		for i := range in.Items {
			ordered.indices[i] = i
		}
		sort.Stable(ordered)

		config := FromConfig(ctx)
		nodes := ordered.limit(config.GlobalEventsTarget, config.GlobalEventsCap)
		out := &model.EventConnection{
			Nodes:      nodes,
			TotalCount: len(nodes),
		}

		return *out, nil
	}

	out := &model.EventConnection{
		Nodes: make([]model.Event, 0),
	}

	// NOTE(negz): The cache implementation we use has only basic support for
	// field selectors, so we just filter our results here. Using the cache's
	// rudimentary field selector support would require us to predeclare a set
	// of event fields to index at cache load time, and even then we could only
	// filter lists by a single field.
	for i := range in.Items {
		e := &in.Items[i] // To avoid taking the address of the range var.

		// This event does not pertain to the involved object.
		if !involves(e, obj) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetEvent(e))
		out.TotalCount++
	}

	sort.Stable(sort.Reverse(out))
	return *out, nil
}

func involves(e *corev1.Event, ref *corev1.ObjectReference) bool {
	// The supplied object won't always have a UID, but the the event's object
	// reference should. This test should be sufficient for most resolvers; the
	// following logic is mostly for the case in which we're looking up events
	// for a ReferenceID (which doesn't include the UID).
	if ref.UID != "" {
		return ref.UID == e.InvolvedObject.UID
	}

	// Note that because we don't know the supplied object's UID from here-on
	// out we can't be sure whether we're returning events for the supplied
	// object, or a different object with the same group, kind, namespace, and
	// name from some time in the past.

	got := ref
	want := e.InvolvedObject

	switch {
	case got.APIVersion != want.APIVersion:
		return false
	case got.Kind != want.Kind:
		return false
	case got.Namespace != want.Namespace:
		return false
	case got.Name != want.Name:
		return false
	default:
		return true
	}
}

type event struct {
	clients ClientCache
}

func (r *event) InvolvedObject(ctx context.Context, obj *model.Event) (model.KubernetesResource, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	u := &unstructured.Unstructured{}
	u.SetAPIVersion(obj.InvolvedObjectRef.APIVersion)
	u.SetKind(obj.InvolvedObjectRef.Kind)
	nn := types.NamespacedName{
		Namespace: obj.InvolvedObjectRef.Namespace,
		Name:      obj.InvolvedObjectRef.Name,
	}
	if err := c.Get(ctx, nn, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetInvolved))
		return nil, nil
	}

	out, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelInvolved))
		return nil, nil
	}
	return out, nil
}

type timeOrderedEventIndices struct {
	indices []int
	items   []corev1.Event
}

func (i timeOrderedEventIndices) Len() int {
	return len(i.indices)
}

func (i timeOrderedEventIndices) Less(a, b int) bool {
	return i.items[i.indices[a]].LastTimestamp.Before(&i.items[i.indices[b]].LastTimestamp)
}

func (i timeOrderedEventIndices) Swap(a, b int) {
	i.indices[a], i.indices[b] = i.indices[b], i.indices[a]
}

// limit tries to return target many events, if these are all warnings. It will
// return more events until it found that many warnings. It stops at the upper
// bound.
//
// Background: we want to fill an event view in the UI with a subset of the
// events if there are many. The UI can switch to warnings-only view, so we
// have to ensure enough warnings such that view is filled.
func (ei timeOrderedEventIndices) limit(target, upperBound int) []model.Event {
	warnings := 0
	nodes := make([]model.Event, 0, upperBound)
	for _, i := range ei.indices {
		nodes = append(nodes, model.GetEvent(&ei.items[i]))

		if len(nodes) >= upperBound {
			// enough events of any type, we can stop now
			break
		}

		if isWarning := ei.items[i].Type == corev1.EventTypeWarning; isWarning {
			warnings++
		}
		if warnings >= target {
			// enough warnings, we can stop now
			break
		}
	}

	return nodes
}
