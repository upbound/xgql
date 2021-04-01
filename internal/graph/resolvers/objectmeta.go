package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
	"github.com/upbound/xgql/internal/unstructured"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

type objectMetaResolver struct {
	clients ClientCache
}

func (r *objectMetaResolver) Owners(ctx context.Context, obj *model.ObjectMeta, controller *bool) (*model.OwnerConnection, error) { //nolint:gocyclo
	// TODO(negz): This method is way over our complexity goal, mostly due to
	// the big switch. Break it out?

	if len(obj.OwnerReferences) == 0 {
		return &model.OwnerConnection{}, nil
	}

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	out := &model.OwnerConnection{
		Items: make([]model.Owner, 0, len(obj.OwnerReferences)),
		Count: len(obj.OwnerReferences),
	}

	for _, ref := range obj.OwnerReferences {
		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringPtrDerefOr(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot get owner"))
			return nil, nil
		}

		owner := model.Owner{Controller: ref.Controller}

		switch {
		case unstructured.ProbablyManaged(u):
			if owner.Resource, err = model.GetManagedResource(u); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model managed resource"))
				continue
			}
		case unstructured.ProbablyProviderConfig(u):
			if owner.Resource, err = model.GetProviderConfig(u); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model provider config"))
				continue
			}
		case unstructured.ProbablyComposite(u):
			if owner.Resource, err = model.GetCompositeResource(u); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model composite resource"))
				continue
			}
		case u.GroupVersionKind() == pkgv1.ProviderGroupVersionKind:
			p := &pkgv1.Provider{}
			if err := convert(u, p); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot convert provider"))
				continue
			}
			if owner.Resource, err = model.GetProvider(p); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model provider"))
				continue
			}
		case u.GroupVersionKind() == pkgv1.ProviderRevisionGroupVersionKind:
			pr := &pkgv1.ProviderRevision{}
			if err := convert(u, pr); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot convert provider revision"))
				continue
			}
			if owner.Resource, err = model.GetProviderRevision(pr); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model provider revision"))
				continue
			}
		case u.GroupVersionKind() == pkgv1.ConfigurationGroupVersionKind:
			c := &pkgv1.Configuration{}
			if err := convert(u, c); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot convert configuration"))
				continue
			}
			if owner.Resource, err = model.GetConfiguration(c); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model configuration"))
				continue
			}
		case u.GroupVersionKind() == pkgv1.ConfigurationRevisionGroupVersionKind:
			cr := &pkgv1.ConfigurationRevision{}
			if err := convert(u, cr); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot convert configuration revision"))
				continue
			}
			if owner.Resource, err = model.GetConfigurationRevision(cr); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model configuration revision"))
				continue
			}
		case u.GroupVersionKind() == extv1.CompositeResourceDefinitionGroupVersionKind:
			xrd := &extv1.CompositeResourceDefinition{}
			if err := convert(u, xrd); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot convert composite resource definition"))
				continue
			}
			if owner.Resource, err = model.GetCompositeResourceDefinition(xrd); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model composite resource definition"))
				continue
			}
		default:
			if owner.Resource, err = model.GetGenericResource(u); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, "cannot model Kubernetes resource"))
				continue
			}
		}

		// There can be only one controller reference.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if pointer.BoolPtrDerefOr(controller, false) && pointer.BoolPtrDerefOr(ref.Controller, false) {
			return &model.OwnerConnection{Items: []model.Owner{owner}, Count: 1}, nil
		}

		out.Items = append(out.Items, owner)
	}

	return out, nil
}

func convert(from *kunstructured.Unstructured, to runtime.Object) error {
	c := runtime.DefaultUnstructuredConverter
	if err := c.FromUnstructured(from.Object, to); err != nil {
		return errors.Wrap(err, "could not convert unstructured object")
	}
	// For whatever reason the *Unstructured's GVK doesn't seem to make it
	// through the conversion process.
	gvk := schema.FromAPIVersionAndKind(from.GetAPIVersion(), from.GetKind())
	to.GetObjectKind().SetGroupVersionKind(gvk)
	return nil
}
