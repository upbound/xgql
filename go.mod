module github.com/upbound/xgql

go 1.16

require (
	github.com/99designs/gqlgen v0.14.1-0.20211024211745-3bbc2a342fc7
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0
	github.com/crossplane/crossplane v1.2.1
	github.com/crossplane/crossplane-runtime v0.13.0
	github.com/go-chi/chi/v5 v5.0.3
	github.com/google/go-cmp v0.5.6
	github.com/kjk/smaz v0.0.0-20151202183815-c61c680e82ff
	github.com/pkg/errors v0.9.1
	github.com/vektah/gqlparser/v2 v2.2.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.25.0
	go.opentelemetry.io/contrib/instrumentation/runtime v0.25.0
	go.opentelemetry.io/otel v1.0.1
	go.opentelemetry.io/otel/exporters/jaeger v1.0.1
	go.opentelemetry.io/otel/exporters/prometheus v0.24.0
	go.opentelemetry.io/otel/metric v0.24.0
	go.opentelemetry.io/otel/sdk v1.0.1
	go.opentelemetry.io/otel/sdk/export/metric v0.24.0
	go.opentelemetry.io/otel/sdk/metric v0.24.0
	go.opentelemetry.io/otel/trace v1.0.1
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.8.3
)
