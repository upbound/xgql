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

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errGetOwner   = "cannot get owner"
	errModelOwner = "cannot model owner"
)

type objectMeta struct {
	clients ClientCache
}

func (r *objectMeta) Owners(ctx context.Context, obj *model.ObjectMeta) (model.OwnerConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.OwnerConnection{}, nil
	}

	owners := make([]model.Owner, 0, len(obj.OwnerReferences))
	selectedFields := GetSelectedFields(ctx).Sub("nodes.resource")
	for _, ref := range obj.OwnerReferences {
		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringDeref(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errGetOwner))
			continue
		}

		kr, err := model.GetKubernetesResource(u, selectedFields)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelOwner))
			continue
		}

		owners = append(owners, model.Owner{Controller: ref.Controller, Resource: kr})
	}

	return model.OwnerConnection{Nodes: owners, TotalCount: len(owners)}, nil
}

func (r *objectMeta) Controller(ctx context.Context, obj *model.ObjectMeta) (model.KubernetesResource, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	selectedFields := GetSelectedFields(ctx)
	for _, ref := range obj.OwnerReferences {
		if !pointer.BoolDeref(ref.Controller, false) {
			continue
		}

		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringDeref(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errGetOwner))
			return nil, nil
		}

		kr, err := model.GetKubernetesResource(u, selectedFields)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelOwner))
			return nil, nil
		}
		return kr, nil
	}

	return nil, nil
}
