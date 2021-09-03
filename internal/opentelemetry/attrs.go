// Copyright 2021 Upbound Inc
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

package opentelemetry

import (
	"go.opentelemetry.io/otel/attribute"
)

const (
	operation = attribute.Key("crossplane.io/gql-operation")
	object    = attribute.Key("crossplane.io/gql-object")
	field     = attribute.Key("crossplane.io/gql-field")
	success   = attribute.Key("crossplane.io/gql-success")
	query     = attribute.Key("crossplane.io/gql-query")
	path      = attribute.Key("crossplane.io/gql-path")
	alias     = attribute.Key("crossplane.io/gql-alias")
)

func variable(v string) attribute.Key { return attribute.Key("crossplane.io/gql-variable/" + v) }
func argument(a string) attribute.Key { return attribute.Key("crossplane.io/gql-argument/" + a) }
