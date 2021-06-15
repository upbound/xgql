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
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// A Tracer that exports OpenTelemetry traces.
type Tracer struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = Tracer{}

// OpenTelemetry tracer.
var tracer = otel.GetTracerProvider().Tracer("crossplane.io/xgql")

// ExtensionName of this extension.
func (t Tracer) ExtensionName() string {
	return "OpenTelemetryTracing"
}

// Validate this extension (a no-op).
func (t Tracer) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptResponse to produce traces.
func (t Tracer) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if !graphql.HasOperationContext(ctx) {
		return next(ctx)
	}

	oc := graphql.GetOperationContext(ctx)
	ctx, span := tracer.Start(ctx, operationName(oc), trace.WithAttributes(query.String(oc.RawQuery)))
	defer span.End()
	if !span.IsRecording() {
		return next(ctx)
	}

	for k, v := range oc.Variables {
		span.SetAttributes(variable(k).String(fmt.Sprintf("%+v", v)))
	}

	return next(ctx)
}

// InterceptField to produce traces.
func (t Tracer) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil {
		return next(ctx)
	}

	ctx, span := tracer.Start(ctx, fc.Object+"/"+fc.Field.Name, trace.WithAttributes(
		path.String(fc.Path().String()),
		object.String(fc.Object),
		field.String(fc.Field.Name),
		alias.String(fc.Field.Alias),
	))
	defer span.End()
	if !span.IsRecording() {
		return next(ctx)
	}

	for _, arg := range fc.Field.Arguments {
		span.SetAttributes(argument(arg.Name).String(arg.Value.String()))
	}

	rsp, err := next(ctx)
	if errs := graphql.GetFieldErrors(ctx, fc); err != nil {
		span.SetStatus(codes.Error, errs.Error())
		span.SetAttributes(success.Bool(false))
	}

	return rsp, err
}

func operationName(oc *graphql.OperationContext) string {
	// oc.OperationName could come as empty here, causing following error if gcp cloud trace enabled:
	//   Failed to export to Stackdriver: rpc error: code = InvalidArgument desc = Missing span display name!
	// We always need a name for spans to get exported to Stackdriver.
	n := "nameless-operation"
	if oc.OperationName != "" {
		n = oc.OperationName
	}
	return n
}
