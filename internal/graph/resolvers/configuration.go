package resolvers

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

const (
	errListConfigRevs = "cannot list configuration revisions"
	errModelConfigRev = "cannot model configuration revision"
	errGetXRD         = "cannot get composite resource definition"
	errModelXRD       = "cannot model composite resource definition"
	errGetComp        = "cannot get composition"
	errModelComp      = "cannot model composition"
)

type configuration struct {
	clients ClientCache
}

func (r *configuration) Events(ctx context.Context, obj *model.Configuration) (*model.EventConnection, error) {
	return nil, nil
}

func (r *configuration) Revisions(ctx context.Context, obj *model.Configuration, active *bool) (*model.ConfigurationRevisionConnection, error) { //nolint:gocyclo
	// NOTE(negz): This method is a little over our complexity goal. Be wary of
	// making it more complex.

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ConfigurationRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigRevs))
		return nil, nil
	}

	out := &model.ConfigurationRevisionConnection{
		Items: make([]model.ConfigurationRevision, 0),
	}

	for i := range in.Items {
		cr := in.Items[i] // So we don't take the address of a range variable.

		// We're not the controller reference of this ConfigurationRevision;
		// it's not one of ours.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if c := metav1.GetControllerOf(&cr); c == nil || c.UID != types.UID(obj.Metadata.UID) {
			continue
		}

		// We only want the active PackageRevision, and this isn't it.
		if pointer.BoolPtrDerefOr(active, false) && cr.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		i, err := model.GetConfigurationRevision(&cr)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelConfigRev))
		}

		out.Items = append(out.Items, i)
		out.Count++
	}

	return out, nil
}

type configurationRevision struct {
	clients ClientCache
}

func (r *configurationRevision) Events(ctx context.Context, obj *model.ConfigurationRevision) (*model.EventConnection, error) {
	return nil, nil
}

type configurationRevisionStatus struct {
	clients ClientCache
}

func (r *configurationRevisionStatus) Objects(ctx context.Context, obj *model.ConfigurationRevisionStatus) (*model.KubernetesResourceConnection, error) { //nolint:gocyclo
	// TODO(negz): This method is over our complexity goal. Maybe break the
	// switch out into its own function?

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	out := &model.KubernetesResourceConnection{
		Items: make([]model.KubernetesResource, 0, len(obj.ObjectRefs)),
	}

	for _, ref := range obj.ObjectRefs {
		// Crossplane lints configuration packages to ensure they only contain XRDs and Compositions
		// but this isn't enforced at the API level. We filter out anything that
		// isn't a CRD, just in case.
		if strings.Split(ref.APIVersion, "/")[0] != extv1.Group {
			continue
		}

		switch ref.Kind {
		case extv1.CompositeResourceDefinitionKind:
			xrd := &extv1.CompositeResourceDefinition{}
			if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, xrd); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, errGetXRD))
				continue
			}

			i, err := model.GetCompositeResourceDefinition(xrd)
			if err != nil {
				graphql.AddError(ctx, errors.Wrap(err, errModelXRD))
				continue
			}

			out.Items = append(out.Items, i)
			out.Count++
		case extv1.CompositionKind:
			cmp := &extv1.Composition{}
			if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, cmp); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, errGetComp))
				continue
			}

			i, err := model.GetComposition(cmp)
			if err != nil {
				graphql.AddError(ctx, errors.Wrap(err, errModelComp))
				continue
			}

			out.Items = append(out.Items, i)
			out.Count++
		}
	}

	return out, nil
}
