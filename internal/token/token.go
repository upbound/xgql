package token

import (
	"context"
	"net/http"
	"strings"
)

type ctxkey int

var key ctxkey

const (
	header = "Authorization"
	prefix = "Bearer"
)

// Middleware extracts a bearer token from the Authorization HTTP header and
// stashes it in the request's context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.Split(r.Header.Get(header), " ")
		if len(h) != 2 || h[0] != prefix {
			// This doesn't seem to be a bearer token.
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key, h[1])))
	})
}

// FromContext extracts a bearer token from the supplied context.
func FromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(key).(string)
	return token, ok
}
