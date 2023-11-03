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

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/99designs/gqlgen/graphql"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/unstructured"
)

type providerConfig struct {
	clients ClientCache
}

func (r *providerConfig) Events(ctx context.Context, obj *model.ProviderConfig) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *providerConfig) Definition(ctx context.Context, obj *model.ProviderConfig) (model.ProviderConfigDefinition, error) { //nolint:gocyclo
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

			out := model.GetCustomResourceDefinition(&crd)
			return &out, nil
		}
	}

	// We found a CRD, let's double check the Group and Kind match our
	// expectations.
	if in.GetSpecGroup() == gv.Group && in.GetSpecNames().Kind == obj.Kind {
		out := model.GetCustomResourceDefinition(in)
		return &out, nil
	}

	return nil, nil
}
