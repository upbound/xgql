package resolvers

import (
	"context"

	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
	"github.com/upbound/xgql/internal/unstructured"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

type objectMetaResolver struct {
	clients   ClientCache
	converter ObjectConvertor
}

func (r *objectMetaResolver) Owners(ctx context.Context, obj *model.ObjectMeta, limit *int, controller *bool) (*model.OwnerConnection, error) { //nolint:gocyclo
	// TODO(negz): This method is way over our complexity goal, mostly due to
	// the big switch. Break it out?

	if len(obj.OwnerReferences) == 0 {
		return &model.OwnerConnection{}, nil
	}

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
	}

	owners := make([]model.Owner, len(obj.OwnerReferences))
	for i, ref := range obj.OwnerReferences {
		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringPtrDerefOr(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			return nil, errors.Wrap(err, "cannot get owner")
		}

		owner := model.Owner{Controller: ref.Controller}

		switch {
		case unstructured.ProbablyManaged(u):
			if owner.Resource, err = model.GetManagedResource(u); err != nil {
				return nil, errors.Wrap(err, "cannot model managed resource")
			}
		case unstructured.ProbablyProviderConfig(u):
			if owner.Resource, err = model.GetProviderConfig(u); err != nil {
				return nil, errors.Wrap(err, "cannot model provider config")
			}
		case unstructured.ProbablyComposite(u):
			if owner.Resource, err = model.GetCompositeResource(u); err != nil {
				return nil, errors.Wrap(err, "cannot model composite resource")
			}
		case u.GroupVersionKind() == pkgv1.ProviderGroupVersionKind:
			p := &pkgv1.Provider{}
			if err := r.converter.Convert(u, p, nil); err != nil {
				return nil, errors.Wrap(err, "cannot convert provider")
			}
			if owner.Resource, err = model.GetProvider(p); err != nil {
				return nil, errors.Wrap(err, "cannot model provider")
			}
		case u.GroupVersionKind() == pkgv1.ProviderRevisionGroupVersionKind:
			pr := &pkgv1.ProviderRevision{}
			if err := r.converter.Convert(u, pr, nil); err != nil {
				return nil, errors.Wrap(err, "cannot convert provider revision")
			}
			if owner.Resource, err = model.GetProviderRevision(pr); err != nil {
				return nil, errors.Wrap(err, "cannot model provider revision")
			}
		case u.GroupVersionKind() == pkgv1.ConfigurationGroupVersionKind:
			c := &pkgv1.Configuration{}
			if err := r.converter.Convert(u, c, nil); err != nil {
				return nil, errors.Wrap(err, "cannot convert configuration")
			}
			if owner.Resource, err = model.GetConfiguration(c); err != nil {
				return nil, errors.Wrap(err, "cannot model configuration")
			}
		case u.GroupVersionKind() == pkgv1.ConfigurationRevisionGroupVersionKind:
			cr := &pkgv1.ConfigurationRevision{}
			if err := r.converter.Convert(u, cr, nil); err != nil {
				return nil, errors.Wrap(err, "cannot convert configuration revision")
			}
			if owner.Resource, err = model.GetConfigurationRevision(cr); err != nil {
				return nil, errors.Wrap(err, "cannot model configuration revision")
			}
		case u.GroupVersionKind() == extv1.CompositeResourceDefinitionGroupVersionKind:
			xrd := &extv1.CompositeResourceDefinition{}
			if err := r.converter.Convert(u, xrd, nil); err != nil {
				return nil, errors.Wrap(err, "cannot convert composite resource definition")
			}
			if owner.Resource, err = model.GetCompositeResourceDefinition(xrd); err != nil {
				return nil, errors.Wrap(err, "cannot model composite resource definition")
			}
		default:
			if owner.Resource, err = model.GetGenericResource(u); err != nil {
				return nil, errors.Wrap(err, "cannot model Kubernetes resource")
			}
		}

		// There can be only one controller reference.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if pointer.BoolPtrDerefOr(controller, false) && pointer.BoolPtrDerefOr(ref.Controller, false) {
			return &model.OwnerConnection{Items: []model.Owner{owner}, Count: 1}, nil
		}

		owners[i] = owner
	}

	if limit != nil && *limit < len(owners) {
		owners = owners[:*limit]
	}

	return &model.OwnerConnection{Items: owners, Count: len(obj.OwnerReferences)}, nil
}
