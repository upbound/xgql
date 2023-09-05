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
	"bytes"
	"context"
	_ "embed"
	"fmt"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/plugin"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	extName          = "LiveQuery"
	fieldName        = "liveQuery"
	typeQuery        = "Query"
	typeSubscription = "Subscription"
	argThrottle      = "throttle"
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

func (LiveQuery) Name() string {
	return extName
}

func (LiveQuery) MutateConfig(cfg *config.Config) error {
	builtins := config.TypeMap{
		typeQuery: {
			Model: config.StringList{
				"github.com/99designs/gqlgen/graphql.Marshaler",
			},
		},
	}

	for typeName, entry := range builtins {
		if cfg.Models.Exists(typeName) {
			return fmt.Errorf("%v already exists which must be reserved when LiveQuery is enabled", typeName)
		}
		cfg.Models[typeName] = entry
	}

	return nil
}

//go:embed resolve_live_query.gotpl
var liveQueryTemplate string

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

func (LiveQuery) InjectSourceLate(schema *ast.Schema) *ast.Source {
	if schema.Query == nil {
		return nil
	}

	subscriptionDefinition := `type Subscription {
	"""
	A live query that is updated when the underlying data changes.
	First, the initial data is sent.
	Then, once the underlying data changes, the "patches" extension is updated with a list of patches to apply to the data.
	"""
	liveQuery(
		"""
		Propose a desired throttle interval ot the server to receive updates to at most once per \"throttle\" milliseconds.
		"""
		throttle: Int = 200
	): Query
}`

	if schema.Subscription != nil {
		subscriptionDefinition = `extend ` + subscriptionDefinition
	}
	return &ast.Source{
		Name:  "live_query/live_query.graphql",
		Input: subscriptionDefinition,
	}
}

func (LiveQuery) ExtensionName() string {
	return extName
}

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
		prevResponse bytes.Buffer
		revision     int
	)
	return func(ctx context.Context) *graphql.Response {
		for {
			resp := handler(ctx)
			if resp == nil {
				cancel()
				return nil
			}
			if prevResponse.Len() == 0 {
				revision++
				_, _ = prevResponse.Write(resp.Data)
				return resp
			}

			diff, err := CreateJSONPatch(prevResponse.Bytes(), resp.Data)
			if err != nil {
				resp.Errors = append(resp.Errors, gqlerror.Wrap(err))
				return nil
			}
			if diff == nil {
				continue
			}
			prevResponse.Reset()
			_, _ = prevResponse.Write(resp.Data)
			resp.Data = nil
			resp.Extensions["patch"] = diff
			return resp
		}
	}
}
