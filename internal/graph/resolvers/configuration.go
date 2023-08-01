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
	"strings"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errListConfigRevs = "cannot list configuration revisions"
	errGetXRD         = "cannot get composite resource definition"
	errGetComp        = "cannot get composition"
)

type configuration struct {
	clients ClientCache
}

func (r *configuration) Events(ctx context.Context, obj *model.Configuration) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *configuration) Revisions(ctx context.Context, obj *model.Configuration) (model.ConfigurationRevisionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.ConfigurationRevisionConnection{}, nil
	}

	in := &pkgv1.ConfigurationRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigRevs))
		return model.ConfigurationRevisionConnection{}, nil
	}

	out := &model.ConfigurationRevisionConnection{
		Nodes: make([]model.ConfigurationRevision, 0),
	}

	for i := range in.Items {
		cr := in.Items[i] // So we don't take the address of a range variable.

		// We're not the controller reference of this ConfigurationRevision;
		// it's not one of ours.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if c := metav1.GetControllerOf(&cr); c == nil || c.UID != types.UID(obj.Metadata.UID) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetConfigurationRevision(&cr))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *configuration) ActiveRevision(ctx context.Context, obj *model.Configuration) (*model.ConfigurationRevision, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ConfigurationRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigRevs))
		return nil, nil
	}

	for i := range in.Items {
		cr := in.Items[i] // So we don't take the address of a range variable.

		// This revision is not active.
		if cr.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		// We're not the controller reference of this ConfigurationRevision;
		// it's not one of ours.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if c := metav1.GetControllerOf(&cr); c == nil || c.UID != types.UID(obj.Metadata.UID) {
			continue
		}

		out := model.GetConfigurationRevision(&cr)
		return &out, nil
	}

	return nil, nil
}

type configurationRevision struct {
	clients ClientCache
}

func (r *configurationRevision) Events(ctx context.Context, obj *model.ConfigurationRevision) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

type configurationRevisionStatus struct {
	clients ClientCache
}

func (r *configurationRevisionStatus) Objects(ctx context.Context, obj *model.ConfigurationRevisionStatus) (model.KubernetesResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.KubernetesResourceConnection{}, nil
	}

	out := &model.KubernetesResourceConnection{
		Nodes: make([]model.KubernetesResource, 0, len(obj.ObjectRefs)),
	}

	// Collect all concurrently.
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, ref := range obj.ObjectRefs {
		// Crossplane lints configuration packages to ensure they only contain XRDs and Compositions
		// but this isn't enforced at the API level. We filter out anything that
		// isn't a CRD, just in case.
		if strings.Split(ref.APIVersion, "/")[0] != extv1.Group {
			continue
		}

		ref := ref // So we don't take the address of a range variable.
		wg.Add(1)
		go func() {
			defer wg.Done()
			var kr model.KubernetesResource
			switch ref.Kind {
			case extv1.CompositeResourceDefinitionKind:
				xrd := &extv1.CompositeResourceDefinition{}
				if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, xrd); err != nil {
					graphql.AddError(ctx, errors.Wrap(err, errGetXRD))
					return
				}
				kr = model.GetCompositeResourceDefinition(xrd)
			case extv1.CompositionKind:
				cmp := &extv1.Composition{}
				if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, cmp); err != nil {
					graphql.AddError(ctx, errors.Wrap(err, errGetComp))
					return
				}
				kr = model.GetComposition(cmp)
			default:
				return
			}
			mu.Lock()
			defer mu.Unlock()
			out.Nodes = append(out.Nodes, kr)
			out.TotalCount++
		}()
	}
	wg.Wait()

	return *out, nil
}
