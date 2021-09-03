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
