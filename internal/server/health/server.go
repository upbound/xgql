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
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/xgql/internal"
	"github.com/upbound/xgql/internal/request"
)

// Server is a liveness and readiness server.
func Server(opts internal.HealthOptions, log logging.Logger) (*http.Server, error) {
	r := chi.NewRouter()
	r.Use(chimid.RedirectSlashes)
	r.Use(chimid.RequestLogger(&request.Formatter{Log: log}))
	r.Use(chimid.Compress(5))

	h := New(WithLogger(log))

	r.Get("/livez", h.GetLiveness)
	r.Get("/readyz", h.GetReadiness)

	return &http.Server{
		Handler:           r,
		Addr:              fmt.Sprintf(":%d", opts.HealthPort),
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}
