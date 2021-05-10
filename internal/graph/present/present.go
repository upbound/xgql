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

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	errRBAC = "possible RBAC permissions error"
)

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

// Error 'presents' errors encountered by GraphQL resolvers.
func Error(ctx context.Context, err error) *gqlerror.Error {
	// Most xgql resolvers read from a controller-runtime cache that is hydrated
	// by taking a watch on any type they're asked to read. The cache uses the
	// credentials passed in by the caller, and will never start if those
	// credentials can't take a watch on the desired type for some reason. If
	// the cache hasn't started by the time the resolver's context is cancelled
	// it will return a timeout error with an obtuse message about an "informer
	// failing to sync". The most common reason a cache won't start is because
	// the caller doesn't have RBAC permissions to list or watch the desired
	// type, so we wrap the error with a hint.
	if kerrors.IsTimeout(err) {
		err = wrap(err, errRBAC)
	}

	// ErrorOnPath will 'upgrade' the supplied error to a GraphQL error if it
	// wasn't one already.
	err = graphql.ErrorOnPath(ctx, err)
	return err.(*gqlerror.Error) //nolint:errorlint // We know err will be a *graphql.Error.
}
