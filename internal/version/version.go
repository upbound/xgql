package version

import "net/http"

// Note that the Version string below is overridden at build time by the xgql
// Makefile, using ldflags.

// Version of xgql.
var Version = "unknown"

const (
	header = "X-Xgql-Version"
)

// Middleware injects the xgql version into the HTTP response headers.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(header, Version)
		next.ServeHTTP(w, r)
	})
}

// Handler returns the running xgql version.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(Version))
	})
}
