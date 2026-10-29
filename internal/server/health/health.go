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
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// readinessTimeout bounds the readiness checks, so that an API server that
// accepts connections but never answers turns into a 503 rather than a
// readiness request that hangs.
var readinessTimeout = 5 * time.Second

// A Check reports whether the service can do its job, for example whether it
// can reach the API server it serves.
type Check func(ctx context.Context) error

// GetLiveness gets the servic liveness.
func (h *Probes) GetLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// GetReadiness gets the service readiness: 200 when every check passes, 503
// with the first failure otherwise. Without checks it always answers 200.
func (h *Probes) GetReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	for _, c := range h.checks {
		if err := c(ctx); err != nil {
			h.log.Debug("Readiness check failed", "error", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// Probes indicates the health of a service.
type Probes struct {
	log    logging.Logger
	checks []Check
}

// Opt sets an option on the probes API.
type Opt func(p *Probes)

// WithLogger sets the logger for the probes API.
func WithLogger(l logging.Logger) Opt {
	return func(p *Probes) {
		p.log = l
	}
}

// WithCheck adds a readiness check.
func WithCheck(c Check) Opt {
	return func(p *Probes) {
		p.checks = append(p.checks, c)
	}
}

// New constructs a new probes API.
func New(opts ...Opt) *Probes {
	p := &Probes{
		log: logging.NewNopLogger(),
	}

	for _, o := range opts {
		o(p)
	}
	return p
}
