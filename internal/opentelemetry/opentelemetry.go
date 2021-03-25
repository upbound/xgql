package opentelemetry

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/unit"
)

// A Tracer that exports OpenTelemetry metrics and traces.
type Tracer struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = Tracer{}

// OpenTelemetry metrics.
var (
	meter = global.GetMeterProvider().Meter("crossplane.io/xgql")

	operation = attribute.Key("crossplane.io/qgl-operation")
	status    = attribute.Key("crossplane.io/qgl-operation-status")
	object    = attribute.Key("crossplane.io/gql-object")
	field     = attribute.Key("crossplane.io/gql-field")

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
func (t Tracer) ExtensionName() string {
	return "OpenTelemetry"
}

// Validate this extension (a no-op).
func (t Tracer) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptOperation to produce metrics and traces.
func (t Tracer) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	oc := graphql.GetOperationContext(ctx)
	reqStarted.Add(ctx, 1, operation.String(oc.OperationName))
	return next(ctx)
}

// InterceptResponse to produce metrics and traces.
func (t Tracer) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	// TODO(negz): Better attributes? This isn't boolean - it's more "were any
	// errors encountered".
	s := "succeeded"
	if len(graphql.GetErrors(ctx)) > 0 {
		s = "failed"
	}

	oc := graphql.GetOperationContext(ctx)
	ms := time.Since(oc.Stats.OperationStart).Milliseconds()
	reqCompleted.Add(ctx, 1, operation.String(oc.OperationName), status.String(s))
	reqDuration.Record(ctx, ms, operation.String(oc.OperationName), status.String(s))

	return next(ctx)
}

// InterceptField to produce metrics and traces.
func (t Tracer) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)

	resStarted.Add(ctx, 1, object.String(fc.Object), field.String(fc.Field.Name))

	started := time.Now()
	rsp, err := next(ctx)

	s := "succeeded"
	if err != nil {
		s = "failed"
	}
	ms := time.Since(started).Milliseconds()

	resCompleted.Add(ctx, 1, object.String(fc.Object), field.String(fc.Field.Name), status.String(s))
	resDuration.Record(ctx, ms, object.String(fc.Object), field.String(fc.Field.Name), status.String(s))

	return rsp, err
}
