package resolvers

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

type configuration struct {
	clients ClientCache
}

func (r *configuration) Events(ctx context.Context, obj *model.Configuration, limit *int) (*model.EventConnection, error) {
	return nil, nil
}

func (r *configuration) Revisions(ctx context.Context, obj *model.Configuration, limit *int, active *bool) (*model.ConfigurationRevisionConnection, error) { //nolint:gocyclo
	// NOTE(negz): This method is a little over our complexity goal. Be wary of
	// making it more complex.

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
	}

	in := &pkgv1.ConfigurationRevisionList{}
	if err := c.List(ctx, in); err != nil {
		return nil, errors.Wrap(err, "cannot list configurations")
	}

	out := &model.ConfigurationRevisionConnection{
		Items: make([]model.ConfigurationRevision, 0),
	}

	for i := range in.Items {
		pr := in.Items[i] // So we don't take the address of a range variable.

		// We're not the controller reference of this ConfigurationRevision;
		// it's not one of ours.
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

		raw, err := json.Marshal(pr)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal JSON")
		}

		out.Items = append(out.Items, model.ConfigurationRevision{
			APIVersion: pr.APIVersion,
			Kind:       pr.Kind,
			Metadata:   model.GetObjectMeta(&pr),
			Spec: &model.ConfigurationRevisionSpec{
				DesiredState:                model.PackageRevisionDesiredState(pr.Spec.DesiredState),
				Package:                     pr.Spec.Package,
				PackagePullPolicy:           model.GetPackagePullPolicy(pr.Spec.PackagePullPolicy),
				Revision:                    int(pr.Spec.Revision),
				IgnoreCrossplaneConstraints: pr.Spec.IgnoreCrossplaneConstraints,
				SkipDependencyResolution:    pr.Spec.SkipDependencyResolution,
			},
			Status: &model.ConfigurationRevisionStatus{
				Conditions:            model.GetConditions(pr.Status.Conditions),
				FoundDependencies:     getIntPtr(&pr.Status.FoundDependencies),
				InstalledDependencies: getIntPtr(&pr.Status.InstalledDependencies),
				InvalidDependencies:   getIntPtr(&pr.Status.InvalidDependencies),
				PermissionRequests:    model.GetPolicyRules(pr.Status.PermissionRequests),
				ObjectRefs:            pr.Status.ObjectRefs,
			},
			Raw: string(raw),
		})
	}

	return out, nil
}

type configurationRevision struct {
	clients ClientCache
}

func (r *configurationRevision) Events(ctx context.Context, obj *model.ConfigurationRevision, limit *int) (*model.EventConnection, error) {
	return nil, nil
}

type configurationRevisionStatus struct {
	clients ClientCache
}

func (r *configurationRevisionStatus) Objects(ctx context.Context, obj *model.ConfigurationRevisionStatus, limit *int) (*model.KubernetesResourceConnection, error) { //nolint:gocyclo
	// TODO(negz): This method is over our complexity goal. Maybe break the
	// switch out into its own function?

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
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

		// Currently only two types exist in the apiextensions.crossplane.io API
		// group, so we assume that if we've found a resource in that group it
		// will be handled by the switch on ref.Kind below.
		out.Count++

		// We've hit our limit; we only want to count from hereon out.
		if limit != nil && *limit < out.Count {
			continue
		}

		switch ref.Kind {
		case extv1.CompositeResourceDefinitionKind:
			xrd := &extv1.CompositeResourceDefinition{}
			if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, xrd); err != nil {
				return nil, errors.Wrap(err, "cannot get CompositeResourceDefinition")
			}

			raw, err := json.Marshal(xrd)
			if err != nil {
				return nil, errors.Wrap(err, "could not marshal JSON")
			}

			out.Items = append(out.Items, model.CompositeResourceDefinition{
				APIVersion: xrd.APIVersion,
				Kind:       xrd.Kind,
				Metadata:   model.GetObjectMeta(xrd),
				Spec: &model.CompositeResourceDefinitionSpec{
					Group: xrd.Spec.Group,
					Names: &model.CompositeResourceDefinitionNames{
						Plural:     xrd.Spec.Names.Plural,
						Singular:   &xrd.Spec.Names.Singular,
						ShortNames: xrd.Spec.Names.ShortNames,
						Kind:       xrd.Spec.Names.Kind,
						ListKind:   &xrd.Spec.Names.ListKind,
						Categories: xrd.Spec.Names.Categories,
					},
					ClaimNames:             model.GetCompositeResourceDefinitionClaimNames(xrd.Spec.ClaimNames),
					Versions:               model.GetCompositeResourceDefinitionVersions(xrd.Spec.Versions),
					DefaultCompositionRef:  xrd.Spec.DefaultCompositionRef,
					EnforcedCompositionRef: xrd.Spec.EnforcedCompositionRef,
				},
				Status: &model.CompositeResourceDefinitionStatus{
					Conditions: model.GetConditions(xrd.Status.Conditions),
				},
				Raw: string(raw),
			})
		case extv1.CompositionKind:
			cmp := &extv1.Composition{}
			if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, cmp); err != nil {
				return nil, errors.Wrap(err, "cannot get Composition")
			}

			raw, err := json.Marshal(cmp)
			if err != nil {
				return nil, errors.Wrap(err, "could not marshal JSON")
			}

			out.Items = append(out.Items, model.Composition{
				APIVersion: cmp.APIVersion,
				Kind:       cmp.Kind,
				Metadata:   model.GetObjectMeta(cmp),
				Spec: &model.CompositionSpec{
					CompositeTypeRef: &model.TypeReference{
						APIVersion: cmp.Spec.CompositeTypeRef.APIVersion,
						Kind:       cmp.Spec.CompositeTypeRef.Kind,
					},
					WriteConnectionSecretsToNamespace: cmp.Spec.WriteConnectionSecretsToNamespace,
				},
				Status: &model.CompositionStatus{
					Conditions: model.GetConditions(cmp.Status.Conditions),
				},
				Raw: string(raw),
			})
		}

	}

	return out, nil
}
