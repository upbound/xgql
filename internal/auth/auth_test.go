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

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/rest"
)

func TestCredentialsInject(t *testing.T) {
	token := "toke-one"

	basicUser := "so"
	basicPass := "basic"

	impUser := "imp"
	impGroup := "impish"
	impExtraKey := "Coolness"
	impExtraVal := "very"

	cases := map[string]struct {
		creds Credentials
		cfg   *rest.Config
		want  *rest.Config
	}{
		"EmptyConfig": {
			creds: Credentials{
				BearerToken:   token,
				BasicUsername: basicUser,
				BasicPassword: basicPass,
				Impersonate: Impersonation{
					Username: impUser,
					Groups:   []string{impGroup},
					Extra:    map[string][]string{impExtraKey: {impExtraVal}},
				},
			},
			cfg: &rest.Config{},
			want: &rest.Config{
				BearerToken: token,
				Username:    basicUser,
				Password:    basicPass,
				Impersonate: rest.ImpersonationConfig{
					UserName: impUser,
					Groups:   []string{impGroup},
					Extra:    map[string][]string{impExtraKey: {impExtraVal}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.creds.Inject(tc.cfg)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nc.Inject(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestCredentialsHash(t *testing.T) {
	cases := map[string]struct {
		creds Credentials
		extra []byte
		want  string
	}{
		"CredsOnly": {
			creds: Credentials{
				BearerToken:   "toke-one",
				BasicUsername: "so",
				BasicPassword: "basic",
				Impersonate: Impersonation{
					Username: "imp",
					Groups:   []string{"imps"},
					Extra:    map[string][]string{"coolness": {"very"}},
				},
			},
			want: "077ae566a720ee75c24fe72441962d258909a380d0f1bf9da576e88ca8f871cd",
		},
		"Extra": {
			creds: Credentials{
				BearerToken: "toke-one",
			},
			extra: []byte("coolness"),
			want:  "2d0c7f73540de525665793bb7a8c970ecaaf4c8f9a08920327819803648e4006",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.creds.Hash(tc.extra)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("c.Hash(...): -want, +got:\n%s", diff)
			}
		})
	}

}

func TestMiddleware(t *testing.T) {
	token := "toke-one"

	basicUser := "so"
	basicPass := "basic"

	impUser := "imp"
	impGroup := "impish"
	impExtraKey := "Coolness"
	impExtraVal := "very"

	type want struct {
		c  Credentials
		ok bool
	}

	tests := map[string]struct {
		r    *http.Request
		want want
	}{
		"WithWellFormedBearerHeader": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add("Authorization", "Bearer "+token)
				return r
			}(),
			want: want{
				c: Credentials{
					BearerToken: token,
				},
				ok: true,
			},
		},
		"WithWellFormedBasicHeader": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.SetBasicAuth(basicUser, basicPass)
				return r
			}(),
			want: want{
				c: Credentials{
					BasicUsername: basicUser,
					BasicPassword: basicPass,
				},
				ok: true,
			},
		},
		"WithWellFormedImpersonationHeaders": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add(headerImpersonateUser, impUser)
				r.Header.Add(headerImpersonateGroup, impGroup)
				r.Header.Add(headerPrefixImpersonateExtra+impExtraKey, impExtraVal)
				return r
			}(),
			want: want{
				c: Credentials{
					Impersonate: Impersonation{
						Username: impUser,
						Groups:   []string{impGroup},
						Extra:    map[string][]string{impExtraKey: {impExtraVal}},
					},
				},
				ok: true,
			},
		},
		"WithMalformedAuthzHeaders": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add("Authorization", "wat")
				return r
			}(),
			want: want{
				c:  Credentials{},
				ok: true,
			},
		},
		"WithoutHeaders": {
			r: httptest.NewRequest("GET", "/", nil),
			want: want{
				c:  Credentials{},
				ok: true,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			h := Middleware(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				c, ok := FromContext(r.Context())
				if diff := cmp.Diff(tc.want.c, c); diff != "" {
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
	creds := Credentials{BearerToken: "toke-one"}

	type want struct {
		c  Credentials
		ok bool
	}

	tests := map[string]struct {
		ctx  context.Context
		want want
	}{
		"WithCredentialsValue": {
			ctx: context.WithValue(context.Background(), key, creds),
			want: want{
				c:  creds,
				ok: true,
			},
		},
		"WithStringValue": {
			ctx: context.WithValue(context.Background(), key, "toke-one"),
			want: want{
				c:  Credentials{},
				ok: false,
			},
		},
		"WithIntValue": {
			ctx: context.WithValue(context.Background(), key, 42),
			want: want{
				c:  Credentials{},
				ok: false,
			},
		},
		"WithNoValue": {
			ctx: context.Background(),
			want: want{
				c:  Credentials{},
				ok: false,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c, ok := FromContext(tc.ctx)
			if diff := cmp.Diff(tc.want.c, c); diff != "" {
				t.Errorf("FromContext(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Errorf("FromContext(...): -want, +got:\n%s", diff)
			}
		})
	}
}
