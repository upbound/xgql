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
	"log"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
)

// A MetricEmitter that exports OpenTelemetry metrics.
type MetricEmitter struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = MetricEmitter{}

var (
	reqStarted   api.Int64Counter
	reqCompleted api.Int64Counter
	reqDuration  api.Float64Histogram
	resStarted   api.Int64Counter
	resCompleted api.Int64Counter
	resDuration  api.Float64Histogram
)

// OpenTelemetry metrics.
func init() {
	var err error

	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}
	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	meter := provider.Meter("crossplane.io/xgql")

	reqStarted, err = meter.Int64Counter("request.started.total",
		api.WithDescription("Total number of requests started"),
		api.WithUnit("1"),
	)
	if err != nil {
		panic(err)
	}

	reqCompleted, err = meter.Int64Counter("request.completed.total",
		api.WithDescription("Total number of requests completed"),
		api.WithUnit("1"),
	)
	if err != nil {
		panic(err)
	}

	reqDuration, err = meter.Float64Histogram("request.duration.ms",
		api.WithDescription("The time taken to complete a request"),
		api.WithUnit("ms"),
	)
	if err != nil {
		panic(err)
	}

	resStarted, err = meter.Int64Counter("resolver.started.total",
		api.WithDescription("Total number of resolvers started"),
		api.WithUnit("1"),
	)
	if err != nil {
		panic(err)
	}

	resCompleted, err = meter.Int64Counter("resolver.completed.total",
		api.WithDescription("Total number of resolvers completed"),
		api.WithUnit("1"),
	)
	if err != nil {
		panic(err)
	}

	resDuration, err = meter.Float64Histogram("resolver.duration.ms",
		api.WithDescription("The time taken to resolve a field"),
		api.WithUnit("ms"),
	)
	if err != nil {
		panic(err)
	}
}

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
		reqStarted.Add(ctx, 1, api.WithAttributes(operation.String(oc.OperationName)))
	}
	return next(ctx)
}

// InterceptResponse to produce metrics .
func (t MetricEmitter) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	if graphql.HasOperationContext(ctx) {
		errs := graphql.GetErrors(ctx)
		oc := graphql.GetOperationContext(ctx)
		ms := time.Since(oc.Stats.OperationStart).Milliseconds()
		attrs := api.WithAttributes(operation.String(oc.OperationName), success.Bool(len(errs) > 0))
		reqCompleted.Add(ctx, 1, attrs)
		reqDuration.Record(ctx, float64(ms), attrs)
	}

	return next(ctx)
}

// InterceptField to produce metrics .
func (t MetricEmitter) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil {
		return next(ctx)
	}

	attrs := api.WithAttributes(object.String(fc.Object), field.String(fc.Field.Name))
	resStarted.Add(ctx, 1, attrs)

	started := time.Now()
	rsp, err := next(ctx)

	ms := time.Since(started).Milliseconds()
	errs := graphql.GetFieldErrors(ctx, fc)

	resCompleted.Add(ctx, 1, attrs, api.WithAttributes(success.Bool(errs != nil)))
	resDuration.Record(ctx, float64(ms), attrs, api.WithAttributes(success.Bool(errs != nil)))

	return rsp, err
}
