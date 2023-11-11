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
	"fmt"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
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
type LiveQuery struct{}

var _ interface {
	plugin.Plugin
	plugin.LateSourceInjector
	plugin.ConfigMutator
	plugin.CodeGenerator
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

// GenerateCode implements plugin.CodeGenerator.
func (LiveQuery) GenerateCode(cfg *codegen.Data) error {
	for _, object := range cfg.Objects {
		if object.Name != typeSubscription {
			continue
		}
		for i := range object.Fields {
			if object.Fields[i].Name != fieldName {
				continue
			}
			// remove field from codegen. need to mark it as marshaller to avoid generating marshalling code.
			object.Fields[i].TypeReference.IsMarshaler = true
			object.Fields = append(object.Fields[:i], object.Fields[i+1:]...)
			break
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
