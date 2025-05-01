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
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

type ctxkey int

var key ctxkey

// Bearer token headers.
const (
	headerAuthn  = "Authorization"
	prefixBearer = "Bearer"
)

// Impersonation headers.
const (
	headerImpersonateUser        = "Impersonate-User"
	headerImpersonateGroup       = "Impersonate-Group"
	headerPrefixImpersonateExtra = "Impersonate-Extra-"
	headerXRemoteUser            = "X-Remote-User"
	headerXRemoteGroup           = "X-Remote-Group"
	headerPrefixXRemoteExtra     = "X-Remote-Extra-"
)

// Impersonation specifies a subject to impersonate. Impersonation configuration
// does not constitute credentials; it must be supplied alongside credentials
// for a subject that has been granted RBAC access to impersonate.
type Impersonation struct {
	Username string
	Groups   []string
	Extra    map[string][]string
}

// AuthenticatingProxy represents the user information from the HTTP headers set
// by an authentication proxy in front of xgql.
type AuthenticatingProxy struct {
	Username string
	Groups   []string
	Extra    map[string][]string
}

// Credentials that a caller may pass to xgql in order to authenticate to a
// Kubernetes API server.
type Credentials struct {
	BearerToken         string
	BasicUsername       string
	BasicPassword       string
	Impersonate         Impersonation
	AuthenticatingProxy AuthenticatingProxy
}

// Inject returns a copy of the supplied REST config with credentials injected.
func (c Credentials) Inject(cfg *rest.Config) *rest.Config {
	out := rest.CopyConfig(cfg)
	out.BearerToken = c.BearerToken
	out.Username = c.BasicUsername
	out.Password = c.BasicPassword
	out.Impersonate = rest.ImpersonationConfig{
		UserName: c.Impersonate.Username,
		Groups:   c.Impersonate.Groups,
		Extra:    c.Impersonate.Extra,
	}

	// Add authenticating proxy headers if configured
	if c.AuthenticatingProxy.Username != "" || len(c.AuthenticatingProxy.Groups) > 0 || len(c.AuthenticatingProxy.Extra) > 0 {
		out.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return &authenticatingProxyTransport{
				RoundTripper: rt,
				username:     c.AuthenticatingProxy.Username,
				groups:       c.AuthenticatingProxy.Groups,
				extra:        c.AuthenticatingProxy.Extra,
			}
		})
	}

	return out
}

// authenticatingProxyTransport is a round tripper that
// adds X-Remote-* headers to requests.
type authenticatingProxyTransport struct {
	http.RoundTripper
	username string
	groups   []string
	extra    map[string][]string
}

func (t *authenticatingProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.username != "" {
		req.Header.Set(headerXRemoteUser, t.username)
	}
	for _, group := range t.groups {
		req.Header.Add(headerXRemoteGroup, group)
	}
	for k, v := range t.extra {
		for _, val := range v {
			req.Header.Add(headerPrefixXRemoteExtra+k, val)
		}
	}
	return t.RoundTripper.RoundTrip(req)
}

// Hash returns a SHA-256 hash of the supplied credentials, plus any extra bytes
// that were supplied.
//
//nolint:errcheck // Writing to a hash never returns an error.
func (c Credentials) Hash(extra []byte) string {
	// Groups are unordered which will result in different hashes for the same
	// set of groups.
	gset := sets.New[string]()
	gset.Insert(c.Impersonate.Groups...)
	gset.Insert(c.AuthenticatingProxy.Groups...)
	sortedGroups := sets.List(gset)

	h := sha256.New()
	h.Write([]byte(c.Impersonate.Username))
	h.Write([]byte(c.AuthenticatingProxy.Username))
	for _, g := range sortedGroups {
		h.Write([]byte(g))
	}

	h.Write(extra)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ExtractBearerToken (if any) from the supplied request.
func ExtractBearerToken(r *http.Request) string {
	h := strings.Split(r.Header.Get(headerAuthn), " ")
	if len(h) != 2 || h[0] != prefixBearer {
		return ""
	}
	return h[1]
}

// ExtractImpersonation configuration (if any) from the supplied request.
func ExtractImpersonation(r *http.Request) Impersonation {
	extra := make(map[string][]string)
	for k, v := range r.Header {
		if !strings.HasPrefix(k, headerPrefixImpersonateExtra) {
			continue
		}
		extra[strings.TrimPrefix(k, headerPrefixImpersonateExtra)] = v
	}

	i := Impersonation{
		Username: r.Header.Get(headerImpersonateUser),
		Groups:   r.Header.Values(headerImpersonateGroup),
	}
	if len(extra) > 0 {
		i.Extra = extra
	}

	return i
}

// ExtractAuthenticatingProxy extracts the user info from the HTTP headers set
// by an authenticating proxy (if any).
func ExtractAuthenticatingProxy(r *http.Request) AuthenticatingProxy {
	// if client did not authenticate using a certificate, proxied user info
	// will not be relayed.
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return AuthenticatingProxy{}
	}
	extra := make(map[string][]string)
	for k, v := range r.Header {
		if !strings.HasPrefix(k, headerPrefixXRemoteExtra) {
			continue
		}
		extra[strings.TrimPrefix(k, headerPrefixXRemoteExtra)] = v
	}

	ap := AuthenticatingProxy{
		Username: r.Header.Get(headerXRemoteUser),
		Groups:   r.Header.Values(headerXRemoteGroup),
	}
	if len(extra) > 0 {
		ap.Extra = extra
	}

	return ap
}

// Middleware extracts credentials from the HTTP request and stashes them in its
// context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bu, bp, _ := r.BasicAuth()
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key, Credentials{
			BasicUsername:       bu,
			BasicPassword:       bp,
			BearerToken:         ExtractBearerToken(r),
			Impersonate:         ExtractImpersonation(r),
			AuthenticatingProxy: ExtractAuthenticatingProxy(r),
		})))
	})
}

func WebsocketInit(ctx context.Context, initPayload transport.InitPayload) (context.Context, error) {
	// don't re-initialize credentials from the init payload if present in request headers.
	if cr, ok := FromContext(ctx); ok {
		if cr.BasicUsername != "" || cr.BasicPassword != "" || cr.BearerToken != "" ||
			cr.Impersonate.Username != "" || len(cr.Impersonate.Groups) > 0 || len(cr.Impersonate.Extra) > 0 {
			return ctx, nil
		}
	}
	r := &http.Request{
		Header: make(http.Header),
	}
	for k := range initPayload {
		s := initPayload.GetString(k)
		if s == "" {
			continue
		}
		r.Header.Add(k, s)
	}
	bu, bp, _ := r.BasicAuth()
	return context.WithValue(ctx, key, Credentials{
		BasicUsername:       bu,
		BasicPassword:       bp,
		BearerToken:         ExtractBearerToken(r),
		Impersonate:         ExtractImpersonation(r),
		AuthenticatingProxy: ExtractAuthenticatingProxy(r),
	}), nil
}

// FromContext extracts credentials from the supplied context.
func FromContext(ctx context.Context) (Credentials, bool) {
	c, ok := ctx.Value(key).(Credentials)
	return c, ok
}
