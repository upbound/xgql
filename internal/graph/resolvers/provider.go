package resolvers

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/negz/xgql/internal/graph/model"
	"github.com/negz/xgql/internal/token"
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
		return nil, errors.Wrap(err, "cannot get client")
	}

	in := &pkgv1.ProviderRevisionList{}
	if err := c.List(ctx, in); err != nil {
		return nil, errors.Wrap(err, "cannot list providers")
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

		raw, err := json.Marshal(pr)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal JSON")
		}

		out.Items = append(out.Items, model.ProviderRevision{
			APIVersion: pr.APIVersion,
			Kind:       pr.Kind,
			Metadata:   model.GetObjectMeta(&pr),
			Spec: &model.ProviderRevisionSpec{
				DesiredState:                model.PackageRevisionDesiredState(pr.Spec.DesiredState),
				Package:                     pr.Spec.Package,
				PackagePullPolicy:           model.GetPackagePullPolicy(pr.Spec.PackagePullPolicy),
				Revision:                    int(pr.Spec.Revision),
				IgnoreCrossplaneConstraints: pr.Spec.IgnoreCrossplaneConstraints,
				SkipDependencyResolution:    pr.Spec.SkipDependencyResolution,
			},
			Status: &model.ProviderRevisionStatus{
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
		return nil, errors.Wrap(err, "cannot get client")
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
			return nil, errors.Wrap(err, "cannot get CustomResourceDefinition")
		}

		raw, err := json.Marshal(crd)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal JSON")
		}

		out.Items = append(out.Items, model.CustomResourceDefinition{
			APIVersion: crd.APIVersion,
			Kind:       crd.Kind,
			Metadata:   model.GetObjectMeta(crd),
			Spec: &model.CustomResourceDefinitionSpec{
				Group: crd.Spec.Group,
				Names: &model.CustomResourceDefinitionNames{
					Plural:     crd.Spec.Names.Plural,
					Singular:   &crd.Spec.Names.Singular,
					ShortNames: crd.Spec.Names.ShortNames,
					Kind:       crd.Spec.Names.Kind,
					ListKind:   &crd.Spec.Names.ListKind,
					Categories: crd.Spec.Names.Categories,
				},
				Versions: model.GetCustomResourceDefinitionVersions(crd.Spec.Versions),
			},
			Status: &model.CustomResourceDefinitionStatus{
				Conditions: model.GetCustomResourceDefinitionConditions(crd.Status.Conditions),
			},
			Raw: string(raw),
		})

	}

	return out, nil
}
