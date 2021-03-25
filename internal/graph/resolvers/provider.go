package resolvers

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

type provider struct {
	clients ClientCache
}

func (r *provider) Events(ctx context.Context, obj *model.Provider, limit *int) (*model.EventConnection, error) {
	return nil, nil
}

func (r *provider) Revisions(ctx context.Context, obj *model.Provider, limit *int, active *bool) (*model.ProviderRevisionConnection, error) { //nolint:gocyclo
	// NOTE(negz): This method is a little over our complexity goal. Be wary of
	// making it more complex.

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	in := &pkgv1.ProviderRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot list providers"))
		return nil, nil
	}

	out := &model.ProviderRevisionConnection{
		Items: make([]model.ProviderRevision, 0),
	}

	for i := range in.Items {
		pr := in.Items[i] // So we don't take the address of a range variable.

		// We're not the controller reference of this ProviderRevision; it's not
		// one of ours.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if c := metav1.GetControllerOf(&pr); c == nil || c.UID != types.UID(obj.Metadata.UID) {
			continue
		}

		// We only want the active PackageRevision, and this isn't it.
		if pointer.BoolPtrDerefOr(active, false) && pr.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		out.Count++

		// We've hit our limit; we only want to count from hereon out.
		if limit != nil && *limit < out.Count {
			continue
		}

		i, err := model.GetProviderRevision(&pr)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot model provider revision"))
			continue
		}

		out.Items = append(out.Items, i)
	}

	return out, nil
}

type providerRevision struct {
	clients ClientCache
}

func (r *providerRevision) Events(ctx context.Context, obj *model.ProviderRevision, limit *int) (*model.EventConnection, error) {
	return nil, nil
}

type providerRevisionStatus struct {
	clients ClientCache
}

func (r *providerRevisionStatus) Objects(ctx context.Context, obj *model.ProviderRevisionStatus, limit *int) (*model.KubernetesResourceConnection, error) {
	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	out := &model.KubernetesResourceConnection{
		Items: make([]model.KubernetesResource, 0, len(obj.ObjectRefs)),
	}

	for _, ref := range obj.ObjectRefs {
		// Crossplane lints provider packages to ensure they only contain CRDs,
		// but this isn't enforced at the API level. We filter out anything that
		// isn't a CRD, just in case.
		if ref.Kind != "CustomResourceDefinition" {
			continue
		}
		if strings.Split(ref.APIVersion, "/")[0] != kextv1.GroupName {
			continue
		}

		out.Count++

		// We've hit our limit; we only want to count from hereon out.
		if limit != nil && *limit < out.Count {
			continue
		}

		crd := &kextv1.CustomResourceDefinition{}
		if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, crd); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot get CustomResourceDefinition"))
			continue
		}

		i, err := model.GetCustomResourceDefinition(crd)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot model custom resource definition"))
			continue
		}

		out.Items = append(out.Items, i)
	}

	return out, nil
}
