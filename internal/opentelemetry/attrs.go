package opentelemetry

import "go.opentelemetry.io/otel/attribute"

const (
	operation = attribute.Key("crossplane.io/qgl-operation")
	status    = attribute.Key("crossplane.io/qgl-operation-status")
	object    = attribute.Key("crossplane.io/gql-object")
	field     = attribute.Key("crossplane.io/gql-field")
)
