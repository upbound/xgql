module github.com/upbound/xgql

go 1.16

require (
	github.com/99designs/gqlgen v0.13.0
	github.com/crossplane/crossplane v1.1.0
	github.com/crossplane/crossplane-runtime v0.13.0
	github.com/go-chi/chi/v5 v5.0.1
	github.com/google/addlicense v0.0.0-20210428195630-6d92264d7170
	github.com/google/go-cmp v0.5.5
	github.com/kjk/smaz v0.0.0-20151202183815-c61c680e82ff
	github.com/pkg/errors v0.9.1
	github.com/vektah/gqlparser/v2 v2.1.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.19.0
	go.opentelemetry.io/contrib/instrumentation/runtime v0.19.0
	go.opentelemetry.io/otel v0.19.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.19.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.19.0
	go.opentelemetry.io/otel/metric v0.19.0
	go.opentelemetry.io/otel/sdk v0.19.0
	go.opentelemetry.io/otel/sdk/metric v0.19.0
	go.opentelemetry.io/otel/trace v0.19.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.3
)
