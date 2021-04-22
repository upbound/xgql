package resolvers

import (
	"context"
	"sort"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

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

func (r *events) Resolve(ctx context.Context, obj *corev1.ObjectReference) (*model.EventConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &corev1.EventList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListEvents))
		return nil, nil
	}

	// If no involved object was supplied we want to fetch all events. This may
	// include Kubernetes events that don't pertain to Crossplane.
	if obj == nil {
		out := &model.EventConnection{
			Nodes:      make([]model.Event, 0, len(in.Items)),
			TotalCount: len(in.Items),
		}
		for i := range in.Items {
			out.Nodes = append(out.Nodes, model.GetEvent(&in.Items[i]))
		}

		sort.Stable(sort.Reverse(out))
		return out, nil
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
	return out, nil
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
