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

package request

import (
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/go-chi/chi/v5/middleware"
)

// Formatter provides a request logging formatter for incoming requests.
type Formatter struct{ Log logging.Logger }

// NewLogEntry emits a new log entry that includes request details.
func (f *Formatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &entry{log: f.Log.WithValues(
		"id", middleware.GetReqID(r.Context()),
		"method", r.Method,
		"tls", r.TLS != nil,
		"host", r.Host,
		"uri", r.RequestURI,
		"protocol", r.Proto,
		"remote", r.RemoteAddr,
	)}
}

type entry struct{ log logging.Logger }

func (e *entry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	e.log.Debug("Handled request",
		"status", status,
		"bytes", bytes,
		"duration", elapsed,
	)
}

func (e *entry) Panic(v interface{}, stack []byte) {
	e.log.Debug("Paniced while handling request", "stack", stack, "panic", v)
}
