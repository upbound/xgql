package resolvers

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func (r *provider) Revisions(ctx context.Context, obj *model.Provider, limit *int, active *bool) (*model.ProviderRevisionConnection, error) {
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
				PackagePullSecrets:          pr.Spec.PackagePullSecrets,
			},
			Status: &model.ProviderRevisionStatus{
				Conditions:            model.GetConditions(pr.Status.Conditions),
				FoundDependencies:     getIntPtr(&pr.Status.FoundDependencies),
				InstalledDependencies: getIntPtr(&pr.Status.InstalledDependencies),
				InvalidDependencies:   getIntPtr(&pr.Status.InvalidDependencies),
				PermissionRequests:    model.GetPolicyRules(pr.Status.PermissionRequests),
				ObjectRefs:            pr.Status.ObjectRefs,
			},
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

func (r *providerRevision) Objects(ctx context.Context, obj *model.ProviderRevision, limit *int) (*model.KubernetesResourceConnection, error) {
	return nil, nil
}

type providerRevisionStatus struct {
	clients ClientCache
}

func (r *providerRevisionStatus) Objects(ctx context.Context, obj *model.ProviderRevisionStatus, limit *int) (*model.KubernetesResourceConnection, error) {
	return nil, nil
}
