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

//go:build generate
// +build generate

package main

import (
	"fmt"
	"go/types"
	"io"
	"log"
	"os"
	"strings"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/upbound/xgql/internal/codegen/gqlgen/extensions/live_query"
	"github.com/vektah/gqlparser/v2/ast"
)

// importType imports a type from a string reference.
func importType(typeRef string) types.Type {
	if typeRef[0] == '*' {
		return types.NewPointer(importType(typeRef[1:]))
	}
	if strings.HasPrefix(typeRef, "[]") {
		return types.NewSlice(importType(typeRef[2:]))
	}

	typeSep := strings.LastIndex(typeRef, ".")
	if typeSep == -1 { // builtinType
		return types.Universe.Lookup(typeRef).Type()
	}

	// typeName is <pkg>.<typeName>
	pkgPath := typeRef[:typeSep]
	typeRef = typeRef[typeSep+1:]

	// pkgPath is <pkgPath...>/<pkgName>
	pkgName := pkgPath
	if pathSep := strings.LastIndex(pkgPath, "/"); pathSep != -1 {
		pkgName = pkgPath[pathSep+1:]
	}

	pkg := types.NewPackage(pkgPath, pkgName)
	// gqlgen doesn't use some of the fields, so we leave them 0/nil
	return types.NewNamed(types.NewTypeName(0, pkg, typeRef, nil), nil, nil)

}

// goTypeFieldHook is a mutation function for modelgen plugin.
// It extends gqlgens `goField` directive, allowing to override
// field type or make the field embedded.
//
// For a brief explanation of Field Mutate Hooks read [GQLGen Recipes].
//
// [GQLGen Recipes]: https://gqlgen.com/recipes/modelgen-hook/#fieldmutatehook
func goTypeFieldHook(td *ast.Definition, fd *ast.FieldDefinition, f *modelgen.Field) (*modelgen.Field, error) {
	// Call default hook to proceed standard directives like goField and goTag.
	// You can omit it, if you don't need.
	if f, err := modelgen.DefaultFieldMutateHook(td, fd, f); err != nil {
		return f, err
	}

	c := fd.Directives.ForName("goField")
	if c != nil {
		// override field type with "type".
		if typeRef := c.Arguments.ForName("type"); typeRef != nil {
			f.Type = importType(typeRef.Value.Raw)
		}
		// reset field name if marked with "embed".
		if embed := c.Arguments.ForName("embed"); embed != nil {
			if do, err := embed.Value.Value(nil); err == nil && do.(bool) {
				f.GoName = ""
			}
		}
	}

	return f, nil
}

func main() {
	// Disable log output from gqlgen.
	log.SetOutput(io.Discard)
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	// Attaching the mutation function onto modelgen plugin
	p := modelgen.Plugin{
		FieldHook: goTypeFieldHook,
	}

	err = api.Generate(cfg, api.PrependPlugin(live_query.LiveQuery{}), api.ReplacePlugin(&p))

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}
