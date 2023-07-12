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
	"errors"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
)

func TestError(t *testing.T) {
	errTimeout := kerrors.NewTimeoutError("too slow", 0)
	errNetwork := syscall.ECONNREFUSED
	errNoKindMatch := &meta.NoKindMatchError{}
	errBoom := errors.New("boom")

	gerrTimeout := gqlerror.WrapPath(nil, errTimeout)
	gerrNetwork := gqlerror.WrapPath(nil, errNetwork)
	gerrNoKindMatch := gqlerror.WrapPath(nil, errNoKindMatch)
	gerrBoom := gqlerror.WrapPath(nil, errBoom)
	gqlInput := "input: "

	type args struct {
		ctx context.Context
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   *gqlerror.Error
	}{
		"TimeoutGQLError": {
			reason: "Timeout errors should be decorated with context a hint about RBAC.",
			args: args{
				ctx: context.Background(),
				err: gerrTimeout,
			},
			want: &gqlerror.Error{
				Message: errRBAC + ": " + gerrTimeout.Message,
				Extensions: map[string]interface{}{
					Code:   errTimeout.Status().Code,
					Source: ErrorSourceAPIServer,
					Reason: errTimeout.Status().Reason,
				},
			},
		},
		"NetworkError": {
			reason: "Network related errors should be 'upgraded' to a GQL error. ",
			args: args{
				ctx: context.Background(),
				err: gerrNetwork,
			},
			want: &gqlerror.Error{
				Message: gqlInput + gerrNetwork.Message,
				Extensions: map[string]interface{}{
					Code:   ErrorRetryable,
					Source: ErrorSourceNetwork,
					Type:   "",
				},
			},
		},
		"NotFoundError": {
			reason: "Error types classified as 'Not Found' should be 'upgraded' to a GQL error. ",
			args: args{
				ctx: context.Background(),
				err: gerrNoKindMatch,
			},
			want: &gqlerror.Error{
				Message: gqlInput + gerrNoKindMatch.Message,
				Extensions: map[string]interface{}{
					Code:   ErrorNotFound,
					Source: ErrorSourceAPI,
					Type:   "NoMatchKind",
				},
			},
		},
		"OtherGQLError": {
			reason: "Regular GQL errors should be returned unchanged.",
			args: args{
				ctx: context.Background(),
				err: gerrBoom,
			},
			want: &gqlerror.Error{
				Message: gerrBoom.Message,
				Extensions: map[string]interface{}{
					Source: ErrorSourceUnknown,
				},
			},
		},
		"OtherTimeoutError": {
			reason: "Non-GQL timeout errors should be both decorated and 'upgraded' to a GQL error.",
			args: args{
				ctx: context.Background(),
				err: errTimeout,
			},
			want: &gqlerror.Error{
				Message: errRBAC + ": " + gerrTimeout.Message,
				Extensions: map[string]interface{}{
					Code:   errTimeout.Status().Code,
					Source: ErrorSourceAPIServer,
					Reason: errTimeout.Status().Reason,
				},
			},
		},
		"OtherError": {
			reason: "Non-GQL errors should be 'upgraded' to a GQL error.",
			args: args{
				ctx: context.Background(),
				err: errBoom,
			},
			want: &gqlerror.Error{
				Message: errBoom.Error(),
				Extensions: map[string]interface{}{
					Source: ErrorSourceUnknown,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Error(tc.args.ctx, tc.args.err)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(gqlerror.Error{})); diff != "" {
				t.Errorf("%s\nError(...): -want, +got\n%s", tc.reason, diff)
			}
		})
	}
}
