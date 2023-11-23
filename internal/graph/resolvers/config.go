// Copyright 2023 Upbound Inc
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

package resolvers

import (
	"context"
	"net/http"
)

type Config struct {
	GlobalEventsTarget int
	GlobalEventsCap    int
}

type configKeyType int

const (
	configKey configKeyType = iota
)

func WithConfig(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, configKey, c)
}

func FromConfig(ctx context.Context) *Config {
	c, found := ctx.Value(configKey).(*Config)
	if !found {
		return &Config{
			GlobalEventsTarget: 500,
			GlobalEventsCap:    1000,
		}
	}
	return c
}

func InjectConfig(c *Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithConfig(r.Context(), c)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
