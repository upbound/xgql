package present

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestError(t *testing.T) {
	errTimeout := kerrors.NewTimeoutError("too slow", 0)
	errBoom := errors.New("boom")

	gerrTimeout := gqlerror.WrapPath(nil, errTimeout)
	gerrBoom := gqlerror.WrapPath(nil, errBoom)

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
			},
		},
		"OtherGQLError": {
			reason: "Regular GQL errors should be returned unchanged.",
			args: args{
				ctx: context.Background(),
				err: gerrBoom,
			},
			want: gerrBoom,
		},
		"OtherTimeoutError": {
			reason: "Non-GQL timeout errors should be both decorated and 'upgraded' to a GQL error.",
			args: args{
				ctx: context.Background(),
				err: errTimeout,
			},
			want: &gqlerror.Error{
				Message: errRBAC + ": " + gerrTimeout.Message,
			},
		},
		"OtherError": {
			reason: "Non-GQL errors should be 'upgraded' to a GQL error.",
			args: args{
				ctx: context.Background(),
				err: errBoom,
			},
			want: gerrBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Error(tc.args.ctx, tc.args.err)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nError(...): -want, +got\n%s", tc.reason, diff)
			}
		})
	}
}
