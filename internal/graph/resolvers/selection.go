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

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

// SelectedFields is a set of selected fields.
type SelectedFields map[string]struct{}

// Has returns true if the field is selected.
func (s SelectedFields) Has(path string) bool {
	_, ok := s[path]
	return ok
}

func (s SelectedFields) Sub(prefix string) SelectedFields {
	if len(s) == 0 {
		return nil
	}
	sub := make(map[string]struct{})
	for k := range s {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix && k[len(prefix)] == '.' {
			sub[k[len(prefix)+1:]] = struct{}{}
		}
	}
	return sub
}

func (s SelectedFields) Pre(prefix string) SelectedFields {
	if len(s) == 0 {
		return nil
	}
	pre := make(map[string]struct{})
	for k := range s {
		pre[prefix+"."+k] = struct{}{}
	}
	return pre
}

type ctxKey struct{}

var selectedFieldsCtxKey = ctxKey{}

func WithSelectedFields(ctx context.Context, selection SelectedFields) context.Context {
	return context.WithValue(ctx, selectedFieldsCtxKey, selection)
}

func GetSelectedFields(ctx context.Context) SelectedFields {
	// see if we already have a selection set in context
	if val, ok := ctx.Value(selectedFieldsCtxKey).(SelectedFields); ok {
		return val
	}
	resctx := graphql.GetFieldContext(ctx)
	if resctx == nil { // not in a resolver context
		return SelectedFields{}
	}

	fields := make(map[string]struct{})
	collectSelectedFields(
		graphql.GetOperationContext(ctx),
		fields,
		resctx.Field.Selections,
		"",
	)
	return fields
}

func collectSelectedFields(ctx *graphql.OperationContext, selected map[string]struct{}, selection ast.SelectionSet, prefix string) {
	for _, f := range graphql.CollectFields(ctx, selection, nil) {
		path := fieldPath(prefix, f.Name)
		selected[path] = struct{}{}
		collectSelectedFields(ctx, selected, f.Selections, path)
	}
}

func fieldPath(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}
