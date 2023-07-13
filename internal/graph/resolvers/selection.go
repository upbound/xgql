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
	sub := SelectedFields{}
	for k := range s {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix && k[len(prefix)] == '.' {
			sub[k[len(prefix)+1:]] = struct{}{}
		}
	}
	return sub
}

func GetSelectedFields(ctx context.Context) SelectedFields {
	resctx := graphql.GetFieldContext(ctx)
	if resctx == nil { // not in a resolver context
		return nil
	}

	selection := SelectedFields{}
	collectSelectedFields(
		graphql.GetOperationContext(ctx),
		selection,
		graphql.CollectFields(graphql.GetOperationContext(ctx), resctx.Field.Selections, nil),
		"",
	)
	return selection
}

func collectSelectedFields(ctx *graphql.OperationContext, selection SelectedFields, fields []graphql.CollectedField, prefix string) {
	for _, column := range fields {
		prefixColumn := fieldPath(prefix, column.Name)
		selection[prefixColumn] = struct{}{}
		collectSelectedFields(ctx, selection, graphql.CollectFields(ctx, column.Selections, nil), prefixColumn)
	}
}

func fieldPath(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}
