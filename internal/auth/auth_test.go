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
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

func TestCredentialsInject(t *testing.T) {
	token := "toke-one"

	basicUser := "so"
	basicPass := "basic"

	impUser := "imp"
	impGroup := "impish"
	impExtraKey := "Coolness"
	impExtraVal := "very"

	proxiedUser := "proxied-user"
	proxiedGroup := "proxied-group"
	proxiedExtraKey := "Proxied-Extra-Key"
	proxiedExtraVal := "proxied-extra-val"

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
				AuthenticatingProxy: AuthenticatingProxy{
					Username: proxiedUser,
					Groups:   []string{proxiedGroup},
					Extra:    map[string][]string{proxiedExtraKey: {proxiedExtraVal}},
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
				WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
					return &authenticatingProxyTransport{
						RoundTripper: rt,
						username:     proxiedUser,
						groups:       []string{proxiedGroup},
						extra:        map[string][]string{proxiedExtraKey: {proxiedExtraVal}},
					}
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.creds.Inject(tc.cfg)
			if diff := cmp.Diff(tc.want, got, cmp.Comparer(transportWrapperComparer)); diff != "" {
				t.Errorf("\nc.Inject(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestAuthenticatingProxyTransportRoundTrip(t *testing.T) {
	proxiedUser := "proxied-user"
	proxiedGroup := "proxied-group"
	proxiedExtraKey := "Proxied-Extra-Key"
	proxiedExtraVal := "proxied-extra-val"

	type args struct {
		username string
		groups   []string
		extra    map[string][]string
	}
	type want struct {
		headers http.Header
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"WithUsername": {
			args: args{
				username: proxiedUser,
			},
			want: want{
				headers: headers(headerXRemoteUser, proxiedUser),
			},
		},
		"WithGroups": {
			args: args{
				groups: []string{proxiedGroup},
			},
			want: want{
				headers: headers(headerXRemoteGroup, proxiedGroup),
			},
		},
		"WithExtra": {
			args: args{
				extra: map[string][]string{
					proxiedExtraKey: {proxiedExtraVal},
				},
			},
			want: want{
				headers: headers(headerPrefixXRemoteExtra+proxiedExtraKey, proxiedExtraVal),
			},
		},
		"WithAllFields": {
			args: args{
				username: proxiedUser,
				groups:   []string{proxiedGroup},
				extra: map[string][]string{
					proxiedExtraKey: {proxiedExtraVal},
				},
			},
			want: want{
				headers: headers(headerXRemoteUser, proxiedUser, headerXRemoteGroup, proxiedGroup, headerPrefixXRemoteExtra+proxiedExtraKey, proxiedExtraVal),
			},
		},
		"WithNoFields": {
			want: want{
				headers: headers(),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// we use a mock RoundTripper to make sure the wrapped RoundTripper
			// by authenticatingProxyTransport is actually called and
			// to capture the modified request by the
			// authenticatingProxyTransport.
			var modifiedReq *http.Request
			mockRT := &mockRoundTripper{
				fn: func(req *http.Request) (*http.Response, error) {
					modifiedReq = req.Clone(req.Context())
					return &http.Response{StatusCode: http.StatusOK}, nil
				},
			}

			tp := &authenticatingProxyTransport{
				RoundTripper: mockRT,
				username:     tc.args.username,
				groups:       tc.args.groups,
				extra:        tc.args.extra,
			}

			resp, err := tp.RoundTrip(httptest.NewRequest("GET", "/", nil))
			if err != nil {
				t.Fatalf("Unexpected error from RoundTripper: %v", err)
			}
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected HTTP status OK, got: %v", resp.StatusCode)
			}

			if diff := cmp.Diff(tc.want.headers, modifiedReq.Header); diff != "" {
				t.Errorf("authenticatingProxyTransport.RoundTrip(...): -want headers, +got headers: \n%s", diff)
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
		"TokenAndBasicCredentialsWithImpersonation": {
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
		"ProxiedUserInfoWithImpersonation": {
			creds: Credentials{
				AuthenticatingProxy: AuthenticatingProxy{
					Username: "proxied-user",
					Groups:   []string{"proxied-group"},
					Extra:    map[string][]string{"proxied-extra-key": {"proxied-extra-val"}},
				},
				Impersonate: Impersonation{
					Username: "imp",
					Groups:   []string{"imps"},
					Extra:    map[string][]string{"coolness": {"very"}},
				},
			},
			want: "f3f55d5e1159455dc5a0f4dc653e224f7bcf6a767d8e7d99c4922a32a815aeea",
		},
		"ProxiedUserInfoWithoutImpersonation": {
			creds: Credentials{
				AuthenticatingProxy: AuthenticatingProxy{
					Username: "proxied-user",
					Groups:   []string{"proxied-group"},
					Extra:    map[string][]string{"proxied-extra-key": {"proxied-extra-val"}},
				},
			},
			want: "9c5e23697eb3b66248ff1597814e63ecba45a3c6a659a185fb134d4e932cbb3d",
		},
		"ProxiedMultipleGroupsWithoutImpersonation": {
			creds: Credentials{
				AuthenticatingProxy: AuthenticatingProxy{
					Username: "proxied-user",
					Groups:   []string{"proxied-group", "secondary-group"},
					Extra:    map[string][]string{"proxied-extra-key": {"proxied-extra-val"}},
				},
			},
			want: "643e511e33078236e00523e5505283b798c85f5dbf2b756a6f710714ded1a2d7",
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

	proxiedUser := "proxied-user"
	proxiedGroup := "proxied-group"
	proxiedExtraKey := "Proxied-Extra-Key"
	proxiedExtraVal := "proxied-extra-val"

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
		"WithProxiedUserInfoNoMTLS": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add(headerXRemoteUser, proxiedUser)
				r.Header.Add(headerXRemoteGroup, proxiedGroup)
				r.Header.Add(headerPrefixXRemoteExtra+proxiedExtraKey, proxiedExtraVal)
				return r
			}(),
			want: want{
				c: Credentials{
					AuthenticatingProxy: AuthenticatingProxy{},
				},
				ok: true,
			},
		},
		"WithProxiedUserInfoUsingMTLS": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add(headerXRemoteUser, proxiedUser)
				r.Header.Add(headerXRemoteGroup, proxiedGroup)
				r.Header.Add(headerPrefixXRemoteExtra+proxiedExtraKey, proxiedExtraVal)

				r.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{{}},
				}
				return r
			}(),
			want: want{
				c: Credentials{
					AuthenticatingProxy: AuthenticatingProxy{
						Username: proxiedUser,
						Groups:   []string{proxiedGroup},
						Extra:    map[string][]string{proxiedExtraKey: {proxiedExtraVal}},
					},
				},
				ok: true,
			},
		},
		"WithProxiedUserInfoWithImpersonationUsingMTLS": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add(headerXRemoteUser, proxiedUser)
				r.Header.Add(headerXRemoteGroup, proxiedGroup)
				r.Header.Add(headerPrefixXRemoteExtra+proxiedExtraKey, proxiedExtraVal)

				r.Header.Add(headerImpersonateUser, impUser)
				r.Header.Add(headerImpersonateGroup, impGroup)
				r.Header.Add(headerPrefixImpersonateExtra+impExtraKey, impExtraVal)

				r.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{{}},
				}
				return r
			}(),
			want: want{
				c: Credentials{
					AuthenticatingProxy: AuthenticatingProxy{
						Username: proxiedUser,
						Groups:   []string{proxiedGroup},
						Extra:    map[string][]string{proxiedExtraKey: {proxiedExtraVal}},
					},
					Impersonate: Impersonation{
						Username: impUser,
						Groups:   []string{impGroup},
						Extra:    map[string][]string{impExtraKey: {impExtraVal}},
					},
				},
				ok: true,
			},
		},
		"WithProxiedUserInfoOnlyUsernameUsingMTLS": {
			r: func() *http.Request {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Add(headerXRemoteUser, proxiedUser)

				r.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{{}},
				}
				return r
			}(),
			want: want{
				c: Credentials{
					AuthenticatingProxy: AuthenticatingProxy{
						Username: proxiedUser,
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

func authnProxyTransportComparer(apt1 *authenticatingProxyTransport, apt2 *authenticatingProxyTransport) bool {
	return apt1.username == apt2.username && cmp.Equal(apt1.groups, apt2.groups) && cmp.Equal(apt1.extra, apt2.extra) && cmp.Equal(apt1.RoundTripper, apt2.RoundTripper)
}

func transportWrapperComparer(w1 transport.WrapperFunc, w2 transport.WrapperFunc) bool {
	if w1 == nil && w2 == nil {
		return true
	}
	if w1 == nil || w2 == nil {
		return false
	}

	rt1 := w1(nil)
	rt2 := w2(nil)
	if diff := cmp.Diff(rt1, rt2, cmp.Comparer(authnProxyTransportComparer)); diff != "" {
		return false
	}
	return true
}

func headers(kv ...string) http.Header {
	h := make(http.Header, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		h[kv[i]] = append(h[kv[i]], kv[i+1])
	}
	return h
}

type mockRoundTripper struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}
