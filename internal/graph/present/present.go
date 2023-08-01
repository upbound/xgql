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

package present

import (
	"context"
	"syscall"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errRBAC = "possible RBAC permissions error"
)

// Error extension fields.
const (
	// Source reflects the 'source' of an error.
	Source = "source"

	// Reason reflects the 'reason' for an error.
	Reason = "reason"

	// Code is the error code, if any.
	Code = "code"

	// Type is the error type, if any.
	Type = "type"
)

// An ErrorCode indicates the type of error.
type ErrorCode string

const (
	// ErrorNotFound is an error class that indicates to the caller that the
	// item that was queried was not found.
	ErrorNotFound ErrorCode = "NOT_FOUND_ERROR"
	// ErrorRetryable is an error class that indicates to the caller that they
	// are safe to retry the operation.
	ErrorRetryable ErrorCode = "RETRYABLE_ERROR"
)

// An ErrorSource indicates where an error originated.
type ErrorSource string

// Error sources.
const (
	ErrorSourceAPI       ErrorSource = "API"
	ErrorSourceAPIServer ErrorSource = "APIServer"
	ErrorSourceCache     ErrorSource = "Cache"
	ErrorSourceNetwork   ErrorSource = "Network"
	ErrorSourceUnknown   ErrorSource = "Unknown"
)

type serverError struct {
	Code   ErrorCode
	Reason string
	Source ErrorSource
	Type   string
}

func (r *serverError) Error() string {
	return r.Reason
}

// wrap adds context to a *gqlerror.Error message while maintaining metadata
// such as its ast.Path that would be obfuscated by errors.Wrap.
func wrap(err error, message string) error {
	gerr := &gqlerror.Error{}
	if !errors.As(err, &gerr) {
		return errors.Wrap(err, message)
	}

	gerr.Message = message + ": " + gerr.Message
	return gerr
}

// convert the provided error to a serverError or return the original error
// if the provided error didn't match the expected types.
func convert(err error) error {
	// Errors due to transient failures.
	var cache *cache.ErrCacheNotStarted
	var dead = context.DeadlineExceeded
	var ref = syscall.ECONNREFUSED

	// Errors indicating that the query failed due to a non-existent failure.
	var kind *meta.NoKindMatchError
	var res *meta.NoResourceMatchError

	// APIStatus errors.
	s := kerrors.APIStatus(nil)

	switch {
	case errors.As(err, &s):
		return err
	case errors.As(err, &kind):
		return &serverError{
			Source: ErrorSourceAPI,
			Reason: err.Error(),
			Code:   ErrorNotFound,
			Type:   "NoMatchKind",
		}
	case errors.As(err, &res):
		return &serverError{
			Source: ErrorSourceAPI,
			Reason: err.Error(),
			Code:   ErrorNotFound,
			Type:   "NoMatchResource",
		}
	case errors.As(err, &cache):
		return &serverError{
			Source: ErrorSourceCache,
			Reason: err.Error(),
			Code:   ErrorRetryable,
		}
	case errors.Is(err, dead), errors.As(err, &ref):
		return &serverError{
			Source: ErrorSourceNetwork,
			Reason: err.Error(),
			Code:   ErrorRetryable,
		}
	default:
		return err
	}
}

// Extend an error with GraphQL extensions.
func Extend(ctx context.Context, err error, ext map[string]interface{}) *gqlerror.Error {
	// 'Upgrade' the error to a GraphQL error if it isn't one already. We know
	// the returned error won't be wrapped.
	//nolint:errorlint
	gerr := graphql.ErrorOnPath(ctx, err).(*gqlerror.Error)
	if gerr.Extensions == nil {
		gerr.Extensions = ext
		return gerr
	}
	for k, v := range ext {
		gerr.Extensions[k] = v
	}
	return gerr
}

// Error 'presents' errors encountered by GraphQL resolvers.
func Error(ctx context.Context, err error) *gqlerror.Error {
	s := kerrors.APIStatus(nil)
	var e *serverError

	// convert the error if applicable
	cerr := convert(err)

	switch {
	case errors.As(cerr, &e):
		return Extend(ctx, cerr, map[string]interface{}{
			Source: e.Source,
			Code:   e.Code,
			Type:   e.Type,
		})
	case errors.As(cerr, &s):
		// Most xgql resolvers read from a controller-runtime cache that is hydrated
		// by taking a watch on any type they're asked to read. The cache uses the
		// credentials passed in by the caller, and will never start if those
		// credentials can't take a watch on the desired type for some reason. If
		// the cache hasn't started by the time the resolver's context is cancelled
		// it will return a timeout error with an obtuse message about an "informer
		// failing to sync". The most common reason a cache won't start is because
		// the caller doesn't have RBAC permissions to list or watch the desired
		// type, so we wrap the error with a hint.
		if s.Status().Reason == metav1.StatusReasonTimeout {
			cerr = wrap(cerr, errRBAC)
		}
		return Extend(ctx, cerr, map[string]interface{}{
			Source: ErrorSourceAPIServer,
			Reason: s.Status().Reason,
			Code:   s.Status().Code,
		})
	default:
		return Extend(ctx, cerr, map[string]interface{}{Source: ErrorSourceUnknown})
	}
}
