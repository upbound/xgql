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
	"fmt"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/unstructured"
)

const (
	errListCRDs = "cannot list custom resource definitions"
)

type managedResource struct {
	clients ClientCache
}

func (r *managedResource) Events(ctx context.Context, obj *model.ManagedResource) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *managedResource) Definition(ctx context.Context, obj *model.ManagedResource) (model.ManagedResourceDefinition, error) { //nolint:gocyclo
	// NOTE(tnthornton) this function is not really all that complex at the
	// moment, however we should be wary of future addtions as we are already
	// running into cyclomatic complexity errors.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	gv, err := schema.ParseGroupVersion(obj.APIVersion)
	if err != nil {
		// This should be pretty much impossible - the API server should not
		// return resources with malformed API versions.
		graphql.AddError(ctx, errors.Wrap(err, errMalformedAPIVersion))
		return nil, nil
	}

	name := pluralForm(strings.ToLower(obj.Kind))

	nn := types.NamespacedName{Name: fmt.Sprintf("%s.%s", name, gv.Group)}
	in := unstructured.NewCRD()
	err = c.Get(ctx, nn, in.GetUnstructured())

	if err != nil && !kerrors.IsNotFound(err) {
		graphql.AddError(ctx, errors.Wrap(err, errGetCRD))
		return nil, nil
	}
	selectedFields := GetSelectedFields(ctx)

	// We didn't find the CRD we were looking for, list all CRDs and see if we
	// can find the matching one.
	if kerrors.IsNotFound(err) {
		lin := unstructured.NewCRDList()
		if err := c.List(ctx, lin.GetUnstructuredList()); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errListCRDs))
			return nil, nil
		}

		for i := range lin.Items {
			crd := unstructured.CustomResourceDefinition{Unstructured: lin.Items[i]} // So we don't take the address of a range variable.

			if crd.GetSpecGroup() != gv.Group {
				continue
			}

			if crd.GetSpecNames().Kind != obj.Kind {
				continue
			}

			out := model.GetCustomResourceDefinition(&crd, selectedFields)
			return &out, nil
		}
	}

	// We found a CRD, let's double check the Group and Kind match our
	// expectations.
	if in.GetSpecGroup() == gv.Group && in.GetSpecNames().Kind == obj.Kind {
		out := model.GetCustomResourceDefinition(in, selectedFields)
		return &out, nil
	}

	return nil, nil
}

type managedResourceSpec struct {
	clients ClientCache
}

func (r *managedResourceSpec) ConnectionSecret(ctx context.Context, obj *model.ManagedResourceSpec) (*model.Secret, error) {
	if obj.WriteConnectionSecretToReference == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{
		Namespace: obj.WriteConnectionSecretToReference.Namespace,
		Name:      obj.WriteConnectionSecretToReference.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}
