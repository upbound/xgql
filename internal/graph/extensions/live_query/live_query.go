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

package live_query

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/plugin"
	"github.com/vektah/gqlparser/v2/ast"
)

const (
	// extName is the name of the extension
	extName = "LiveQuery"
	// fieldName is the field name exposing live queries.
	fieldName = "liveQuery"
	// typeQuery is the schema type that will be made subscribeable.
	typeQuery = "Query"
	// typeSubscription is the schema type that will expose "liveQuery" field.
	typeSubscription = "Subscription"
	// argThrottle is the name of the "trottle" argument for the "liveQuery" field.
	argThrottle = "throttle"
)

// LiveQuery is a graphql.HandlerExtension that enables live queries.
type LiveQuery struct {
}

var _ interface {
	plugin.Plugin
	plugin.LateSourceInjector
	plugin.ConfigMutator
	plugin.CodeGenerator
} = LiveQuery{}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
} = LiveQuery{}

// Name implements plugin.Plugin.
func (LiveQuery) Name() string {
	return extName
}

// MutateConfig implements plugin.ConfigMutator
func (LiveQuery) MutateConfig(cfg *config.Config) error {
	// make Query type resolveable as graphql.Marshaler.
	builtins := config.TypeMap{
		typeQuery: {
			Model: config.StringList{
				"github.com/99designs/gqlgen/graphql.Marshaler",
			},
		},
	}

	for typeName, typeEntry := range builtins {
		// TODO(avalanche123): extend query type models list if already exists.
		if cfg.Models.Exists(typeName) {
			return fmt.Errorf("%v already exists which must be reserved when LiveQuery is enabled", typeName)
		}
		cfg.Models[typeName] = typeEntry
	}

	return nil
}

//go:embed resolve_live_query.gotpl
var liveQueryTemplate string

// GenerateCode implements plugin.CodeGenerator.
func (LiveQuery) GenerateCode(cfg *codegen.Data) error {
	for _, object := range cfg.Objects {
		if object.Name != typeSubscription {
			continue
		}
		for _, f := range object.Fields {
			if f.Name != fieldName {
				continue
			}
			f.TypeReference.IsMarshaler = true
			f.IsResolver = false
			f.GoFieldType = codegen.GoFieldMethod
			f.GoReceiverName = "ec"
			f.GoFieldName = "__resolve_liveQuery"
			f.MethodHasContext = true

			return templates.Render(templates.Options{
				PackageName:     cfg.Config.Exec.Package,
				Filename:        cfg.Config.Exec.Dir() + "/resolve_live_query.gen.go",
				Data:            f,
				GeneratedHeader: true,
				Packages:        cfg.Config.Packages,
				Template:        liveQueryTemplate,
			})
		}
	}
	return nil
}

// InjectSourceLate implements plugin.LateSourceInjector.
func (LiveQuery) InjectSourceLate(schema *ast.Schema) *ast.Source {
	if schema.Query == nil {
		return nil
	}
	// subscriptionDefinition is the subscription type with a "liveQuery" field.
	subscriptionDefinition := `type ` + typeSubscription + ` {
		"""
		A live query that is updated when the underlying data changes.
		First, the initial data is sent.
		Then, once the underlying data changes, the "patches" extension is updated with a list of patches to apply to the data.
		"""
		` + fieldName + `(
			"""
			Propose a desired throttle interval ot the server to receive updates to at most once per \"throttle\" milliseconds.
			"""
			` + argThrottle + `: Int = 200
		): ` + typeQuery + `
	}`
	if schema.Subscription != nil {
		subscriptionDefinition = `extend ` + subscriptionDefinition
	}
	return &ast.Source{
		Name:  "live_query/live_query.graphql",
		Input: subscriptionDefinition,
	}
}

// ExtensionName implements graphql.HandlerExtension
func (LiveQuery) ExtensionName() string {
	return extName
}

// Validate implements graphql.HandlerExtension
func (l LiveQuery) Validate(s graphql.ExecutableSchema) error {
	subscriptionType, ok := s.Schema().Types[typeSubscription]
	if !ok {
		return fmt.Errorf("%q type not found", typeSubscription)
	}

	field := subscriptionType.Fields.ForName(fieldName)
	if field == nil {
		return fmt.Errorf("%q type is missing %q field", typeSubscription, fieldName)
	}
	if field.Type.String() != typeQuery {
		return fmt.Errorf("%q field on %q is not of type %q", fieldName, typeSubscription, typeQuery)
	}
	if field.Arguments.ForName(argThrottle) == nil {
		return fmt.Errorf("%q field on %q is missing the %q argument", fieldName, typeSubscription, argThrottle)
	}

	return nil
}

type patch struct {
	Revision  int         `json:"revision"`
	JSONPatch []Operation `json:"jsonPatch,omitempty"`
}

// InterceptOperation implements graphql.OperationInterceptor
func (l LiveQuery) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	oc := graphql.GetOperationContext(ctx)
	if oc.Operation.Operation != ast.Subscription {
		return next(ctx)
	}
	fields := graphql.CollectFields(oc, oc.Operation.SelectionSet, []string{typeSubscription})
	if len(fields) != 1 {
		return next(ctx)
	}
	field := fields[0]
	if field.Name != fieldName {
		return next(ctx)
	}
	ctx, cancel := context.WithCancel(ctx)
	handler := next(ctx)
	var (
		prevData strings.Builder
		revision int
	)
	return func(ctx context.Context) *graphql.Response {
		for {
			resp := handler(ctx)
			if resp == nil {
				cancel()
				return nil
			}
			data := resp.Data
			// Compare new data with previous response.
			if prevData.Len() > 0 {
				diff, err := CreateJSONPatch(prevData.String(), string(data))
				if err != nil {
					cancel()
					panic(err)
				}
				// response is the same, skip it.
				if len(diff) == 0 {
					continue
				}
				// reset data and add patch extension.
				resp.Data = nil
				resp.Extensions["patch"] = patch{
					Revision:  revision,
					JSONPatch: diff,
				}
			}
			revision++
			// keep current data as previous response.
			prevData.Reset()
			_, _ = prevData.Write(data)
			return resp
		}
	}
}
