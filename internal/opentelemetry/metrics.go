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
	"time"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/unit"
)

// A MetricEmitter that exports OpenTelemetry metrics.
type MetricEmitter struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = MetricEmitter{}

// OpenTelemetry metrics.
var (
	meter = global.GetMeterProvider().Meter("crossplane.io/xgql")

	reqStarted = metric.Must(meter).NewInt64Counter("request.started.total",
		metric.WithDescription("Total number of requests started"),
		metric.WithUnit(unit.Dimensionless))

	reqCompleted = metric.Must(meter).NewInt64Counter("request.completed.total",
		metric.WithDescription("Total number of requests completed"),
		metric.WithUnit(unit.Dimensionless))

	reqDuration = metric.Must(meter).NewInt64ValueRecorder("request.duration.ms",
		metric.WithDescription("The time taken to complete a request"),
		metric.WithUnit(unit.Milliseconds))

	resStarted = metric.Must(meter).NewInt64Counter("resolver.started.total",
		metric.WithDescription("Total number of resolvers started"),
		metric.WithUnit(unit.Dimensionless))

	resCompleted = metric.Must(meter).NewInt64Counter("resolver.completed.total",
		metric.WithDescription("Total number of resolvers completed"),
		metric.WithUnit(unit.Dimensionless))

	resDuration = metric.Must(meter).NewInt64ValueRecorder("resolver.duration.ms",
		metric.WithDescription("The time taken to resolve a field"),
		metric.WithUnit(unit.Milliseconds))
)

// ExtensionName of this extension.
func (t MetricEmitter) ExtensionName() string {
	return "OpenTelemetryMetrics"
}

// Validate this extension (a no-op).
func (t MetricEmitter) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptOperation to produce metrics .
func (t MetricEmitter) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	if graphql.HasOperationContext(ctx) {
		oc := graphql.GetOperationContext(ctx)
		reqStarted.Add(ctx, 1, operation.String(oc.OperationName))
	}
	return next(ctx)
}

// InterceptResponse to produce metrics .
func (t MetricEmitter) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if graphql.HasOperationContext(ctx) {
		errs := graphql.GetErrors(ctx)
		oc := graphql.GetOperationContext(ctx)
		ms := time.Since(oc.Stats.OperationStart).Milliseconds()
		reqCompleted.Add(ctx, 1, operation.String(oc.OperationName), success.Bool(len(errs) > 0))
		reqDuration.Record(ctx, ms, operation.String(oc.OperationName), success.Bool(len(errs) > 0))
	}

	return next(ctx)
}

// InterceptField to produce metrics .
func (t MetricEmitter) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil {
		return next(ctx)
	}

	resStarted.Add(ctx, 1, object.String(fc.Object), field.String(fc.Field.Name))

	started := time.Now()
	rsp, err := next(ctx)

	ms := time.Since(started).Milliseconds()
	errs := graphql.GetFieldErrors(ctx, fc)

	resCompleted.Add(ctx, 1, object.String(fc.Object), field.String(fc.Field.Name), success.Bool(errs != nil))
	resDuration.Record(ctx, ms, object.String(fc.Object), field.String(fc.Field.Name), success.Bool(errs != nil))

	return rsp, err
}
