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
	"encoding/json"

	"github.com/99designs/gqlgen/graphql"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errCreateResource        = "cannot create Kubernetes resource"
	errUpdateResource        = "cannot update Kubernetes resource"
	errDeleteResource        = "cannot delete Kubernetes resource"
	errUnmarshalUnstructured = "cannot unmarshal input unstructured JSON"

	errFmtUnmarshalPatch = "cannot unmarshal unstructured patch JSON at index %d"
	errFmtPatch          = "cannot apply patch at index %d"
)

// IsRetriable indicates that an error may succeed if retried.
func IsRetriable(err error) bool { //nolint:gocyclo // It's just a big old switch.
	switch {
	case kerrors.IsTimeout(err):
		return true
	case kerrors.IsServerTimeout(err):
		return true
	case kerrors.IsInternalError(err):
		return true
	case kerrors.IsTooManyRequests(err):
		return true
	case kerrors.IsUnexpectedServerError(err):
		return true
	case kerrors.ReasonForError(err) == v1.StatusReasonUnknown:
		// This error doesn't seem to be from Kubernetes.
		return true
	default:
		return false
	}
}

type mutation struct {
	clients ClientCache
}

func (r *mutation) CreateKubernetesResource(ctx context.Context, input model.CreateKubernetesResourceInput) (model.CreateKubernetesResourcePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.CreateKubernetesResourcePayload{}, nil
	}

	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(input.Unstructured, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errUnmarshalUnstructured))
		return model.CreateKubernetesResourcePayload{}, nil
	}

	pv := fieldpath.Pave(u.Object)
	for i, p := range input.Patches {
		var v interface{}
		if err := json.Unmarshal(p.Unstructured, &v); err != nil {
			graphql.AddError(ctx, errors.Wrapf(err, errFmtUnmarshalPatch, i))
			return model.CreateKubernetesResourcePayload{}, nil
		}
		if err := pv.SetValue(p.FieldPath, v); err != nil {
			graphql.AddError(ctx, errors.Wrapf(err, errFmtPatch, i))
			return model.CreateKubernetesResourcePayload{}, nil
		}
	}

	if err := retry.OnError(retry.DefaultBackoff, IsRetriable, func() error { return c.Create(ctx, u) }); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errCreateResource))
		return model.CreateKubernetesResourcePayload{}, nil
	}

	kr, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return model.CreateKubernetesResourcePayload{}, nil
	}
	return model.CreateKubernetesResourcePayload{Resource: kr}, nil
}

func (r *mutation) UpdateKubernetesResource(ctx context.Context, id model.ReferenceID, input model.UpdateKubernetesResourceInput) (model.UpdateKubernetesResourcePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.UpdateKubernetesResourcePayload{}, nil
	}

	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(input.Unstructured, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errUnmarshalUnstructured))
		return model.UpdateKubernetesResourcePayload{}, nil
	}

	pv := fieldpath.Pave(u.Object)
	for i, p := range input.Patches {
		var v interface{}
		if err := json.Unmarshal(p.Unstructured, &v); err != nil {
			graphql.AddError(ctx, errors.Wrapf(err, errFmtUnmarshalPatch, i))
			return model.UpdateKubernetesResourcePayload{}, nil
		}
		if err := pv.SetValue(p.FieldPath, v); err != nil {
			graphql.AddError(ctx, errors.Wrapf(err, errFmtPatch, i))
			return model.UpdateKubernetesResourcePayload{}, nil
		}
	}

	// We expect the caller to read, modify, then update so the supplied
	// unstructured JSON _should_ already have the correct GVK, namespace, and
	// name. Nonetheless we inject those within the supplied ID just in case.
	u.SetAPIVersion(id.APIVersion)
	u.SetKind(id.Kind)
	u.SetNamespace(id.Namespace)
	u.SetName(id.Name)

	if err := retry.OnError(retry.DefaultBackoff, IsRetriable, func() error { return c.Update(ctx, u) }); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errUpdateResource))
		return model.UpdateKubernetesResourcePayload{}, nil
	}

	kr, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return model.UpdateKubernetesResourcePayload{}, nil
	}
	return model.UpdateKubernetesResourcePayload{Resource: kr}, nil
}

func (r *mutation) DeleteKubernetesResource(ctx context.Context, id model.ReferenceID) (model.DeleteKubernetesResourcePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.DeleteKubernetesResourcePayload{}, nil
	}

	u := &unstructured.Unstructured{}
	u.SetAPIVersion(id.APIVersion)
	u.SetKind(id.Kind)
	u.SetNamespace(id.Namespace)
	u.SetName(id.Name)
	if err := retry.OnError(retry.DefaultBackoff, IsRetriable, func() error { return c.Delete(ctx, u) }); resource.IgnoreNotFound(err) != nil {
		graphql.AddError(ctx, errors.Wrap(err, errDeleteResource))
		return model.DeleteKubernetesResourcePayload{}, nil //nolint:nilerr // IgnoreNotFound appears to trigger this linter.
	}

	kr, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return model.DeleteKubernetesResourcePayload{}, nil
	}
	return model.DeleteKubernetesResourcePayload{Resource: kr}, nil
}
