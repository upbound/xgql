// Copyright 2022 Upbound Inc
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

package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/xgql/internal"
)

func TestGetReadiness(t *testing.T) {
	errBoom := errors.New("boom")
	pass := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errBoom }
	hang := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	cases := map[string]struct {
		reason   string
		checks   []Check
		wantCode int
		wantBody string
	}{
		"NoChecks": {
			reason:   "Without checks readiness is always reported",
			wantCode: http.StatusOK,
		},
		"Passing": {
			reason:   "Readiness is reported when every check passes",
			checks:   []Check{pass, pass},
			wantCode: http.StatusOK,
		},
		"Failing": {
			reason:   "A failing check turns into a 503 that carries its error",
			checks:   []Check{pass, fail},
			wantCode: http.StatusServiceUnavailable,
			wantBody: "boom\n",
		},
		"Hanging": {
			reason:   "A check that never answers is cut off by the timeout",
			checks:   []Check{hang},
			wantCode: http.StatusServiceUnavailable,
			wantBody: context.DeadlineExceeded.Error() + "\n",
		},
	}

	readinessTimeout = 50 * time.Millisecond
	t.Cleanup(func() { readinessTimeout = 5 * time.Second })

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			opts := []Opt{WithLogger(logging.NewNopLogger())}
			for _, c := range tc.checks {
				opts = append(opts, WithCheck(c))
			}
			rec := httptest.NewRecorder()
			New(opts...).GetReadiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if diff := cmp.Diff(tc.wantCode, rec.Code); diff != "" {
				t.Errorf("\n%s\nGetReadiness(...): -want code, +got code:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.wantBody, rec.Body.String()); diff != "" {
				t.Errorf("\n%s\nGetReadiness(...): -want body, +got body:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestServer(t *testing.T) {
	down := errors.New("api server down")
	srv, err := Server(internal.HealthOptions{HealthPort: 0}, logging.NewNopLogger(), func(context.Context) error { return down })
	if err != nil {
		t.Fatalf("Server(...): %v", err)
	}

	cases := map[string]struct {
		path     string
		wantCode int
	}{
		"LivezIgnoresChecks": {path: "/livez", wantCode: http.StatusOK},
		"ReadyzUsesChecks":   {path: "/readyz", wantCode: http.StatusServiceUnavailable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if diff := cmp.Diff(tc.wantCode, rec.Code); diff != "" {
				t.Errorf("GET %s: -want code, +got code:\n%s", tc.path, diff)
			}
		})
	}
}
