package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errModelDefined = "cannot model defined resource"
)

type genericResource struct {
	clients ClientCache
}

func (r *genericResource) Events(ctx context.Context, obj *model.GenericResource) (*model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		Namespace:  pointer.StringPtrDerefOr(obj.Metadata.Namespace, ""),
		UID:        types.UID(obj.Metadata.UID),
	})
}

type secret struct {
	clients ClientCache
}

func (r *secret) Data(_ context.Context, obj *model.Secret, keys []string) ([]*string, error) {
	if obj.Data == nil {
		return nil, nil
	}
	out := make([]*string, len(keys))
	for i := range keys {
		v, ok := obj.Data[keys[i]]
		if !ok {
			continue
		}
		s := string(v)
		out[i] = &s
	}
	return out, nil
}

func (r *secret) Events(ctx context.Context, obj *model.Secret) (*model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		Namespace:  pointer.StringPtrDerefOr(obj.Metadata.Namespace, ""),
		UID:        types.UID(obj.Metadata.UID),
	})
}

type configMap struct {
	clients ClientCache
}

func (r *configMap) Data(_ context.Context, obj *model.ConfigMap, keys []string) ([]*string, error) {
	if obj.Data == nil {
		return nil, nil
	}
	out := make([]*string, len(keys))
	for i := range keys {
		v, ok := obj.Data[keys[i]]
		if !ok {
			continue
		}
		out[i] = &v
	}
	return out, nil
}

func (r *configMap) Events(ctx context.Context, obj *model.ConfigMap) (*model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		Namespace:  pointer.StringPtrDerefOr(obj.Metadata.Namespace, ""),
		UID:        types.UID(obj.Metadata.UID),
	})
}

type crd struct {
	clients ClientCache
}

func (r *crd) Events(ctx context.Context, obj *model.CustomResourceDefinition) (*model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *crd) DefinedResources(ctx context.Context, obj *model.CustomResourceDefinition, version *string) (*model.KubernetesResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	gv := schema.GroupVersion{Group: obj.Spec.Group}
	switch {
	case version != nil:
		gv.Version = *version
	default:
		gv.Version = pickCRDVersion(obj.Spec.Versions)
	}

	in := &kunstructured.UnstructuredList{}
	in.SetAPIVersion(gv.String())
	in.SetKind(obj.Spec.Names.Kind + "List")
	if lk := obj.Spec.Names.ListKind; lk != nil && *lk != "" {
		in.SetKind(*lk)
	}

	// TODO(negz): Support filtering this by a particular namespace? Currently
	// we assume the caller has access to list the defined resource in all
	// namespaces (or at cluster scope). In practice we expect this call to be
	// used by platform operators to list managed resources, which are cluster
	// scoped, but in theory a CRD could define any kind of custom resource. We
	// could accept an optional 'namespace' argument and pass client.InNamespace
	// to c.List. We'd also need to fetch client with a namespaced cache by
	// passing clients.WithNamespace to r.clients.Get above.
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListResources))
		return nil, nil
	}

	out := &model.KubernetesResourceConnection{
		Nodes:      make([]model.KubernetesResource, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		u := in.Items[i]

		kr, err := model.GetKubernetesResource(&u)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelDefined))
		}
		out.Nodes = append(out.Nodes, kr)
	}

	return out, nil
}

// TODO(negz): Try to pick the 'highest' version (e.g. v2 > v1 > v1beta1),
// rather than returning the first served one. There's no guarantee versions
// will actually follow this convention, but it's ubiquitous.
func pickCRDVersion(vs []model.CustomResourceDefinitionVersion) string {
	for _, v := range vs {
		if v.Served {
			return v.Name
		}
	}

	// We shouldn't get here, unless the CRD is serving no versions?
	return ""
}
