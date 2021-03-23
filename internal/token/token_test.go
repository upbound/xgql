package token

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMiddleware(t *testing.T) {
	coolToken := "socool"

	type want struct {
		token string
		ok    bool
	}

	tests := map[string]struct {
		r    *http.Request
		want want
	}{
		"WithWellFormedHeader": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add("Authorization", "Bearer "+coolToken)
				return r
			}(),
			want: want{
				token: coolToken,
				ok:    true,
			},
		},
		"WithMalformedHeader": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add("Authorization", "wat")
				return r
			}(),
			want: want{
				token: "",
				ok:    false,
			},
		},
		"WithoutHeader": {
			r: httptest.NewRequest("GET", "/", nil),
			want: want{
				token: "",
				ok:    false,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			h := Middleware(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				token, ok := FromContext(r.Context())
				if diff := cmp.Diff(tc.want.token, token); diff != "" {
					t.Errorf("FromContext(...): -want, +got:\n%s", diff)
				}
				if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
					t.Errorf("FromContext(...): -want, +got:\n%s", diff)
				}
			}))

			h.ServeHTTP(httptest.NewRecorder(), tc.r)
		})
	}
}

func TestFromContext(t *testing.T) {
	coolToken := "socool"

	type want struct {
		token string
		ok    bool
	}

	tests := map[string]struct {
		ctx  context.Context
		want want
	}{
		"WithStringValue": {
			ctx: context.WithValue(context.Background(), key, coolToken),
			want: want{
				token: coolToken,
				ok:    true,
			},
		},
		"WithIntValue": {
			ctx: context.WithValue(context.Background(), key, 42),
			want: want{
				token: "",
				ok:    false,
			},
		},
		"WithNoValue": {
			ctx: context.Background(),
			want: want{
				token: "",
				ok:    false,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			token, ok := FromContext(tc.ctx)
			if diff := cmp.Diff(tc.want.token, token); diff != "" {
				t.Errorf("FromContext(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Errorf("FromContext(...): -want, +got:\n%s", diff)
			}
		})
	}
}
