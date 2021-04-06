package main

import (
	"io/ioutil"
	stdlog "log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/apollotracing"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"gopkg.in/alecthomas/kingpin.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/present"
	"github.com/upbound/xgql/internal/graph/resolvers"
	"github.com/upbound/xgql/internal/opentelemetry"
	"github.com/upbound/xgql/internal/version"
)

// A set of resources that we never want to cache. Clients take a watch on any
// kind of resource they're asked to read unless it's in this list. We allow
// caching of arbitrary resources (i.e. *unstructured.Unstructured, which may
// have any GVK) in order to allow us to cache managed and composite resources.
// We're particularly at risk of caching resources like these unexpectedly when
// iterating through arrays of arbitrary object references (e.g. owner refs).
var noCache = []client.Object{
	// We don't cache these resources because there's a (very slim) possibility
	// they could end up as the owner reference of a resource we're concerned
	// with, and we don't want to try to watch (e.g.) all pods in the cluster
	// just because a pod somehow became the owner reference of an XR.
	&corev1.Pod{},
	&corev1.ConfigMap{},
	&corev1.Service{},
	&corev1.ServiceAccount{},
	&appsv1.Deployment{},
	&appsv1.DaemonSet{},
	&rbacv1.RoleBinding{},
	&rbacv1.ClusterRoleBinding{},

	// We don't cache secrets because there's a high risk that the caller won't
	// have access to list and watch secrets across all namespaces.
	&corev1.Secret{},
}

func main() {
	var (
		app      = kingpin.New(filepath.Base(os.Args[0]), "A GraphQL API for Crossplane.").DefaultEnvars()
		debug    = app.Flag("debug", "Enable debug logging.").Short('d').Bool()
		listen   = app.Flag("listen", "Address at which to listen for TLS connections. Requires TLS cert and key.").Default(":8443").String()
		tlsCert  = app.Flag("tls-cert", "Path to the TLS certificate file used to serve TLS connections.").ExistingFile()
		tlsKey   = app.Flag("tls-key", "Path to the TLS key file used to serve TLS connections.").ExistingFile()
		insecure = app.Flag("listen-insecure", "Address at which to listen for insecure connections.").Default("127.0.0.1:8080").String()
		play     = app.Flag("enable-playground", "Serve a GraphQL Playground.").Bool()
		agent    = app.Flag("trace-agent", "Address of the Jaeger trace agent. Leave unset to disable tracing.").String()
		ratio    = app.Flag("trace-ratio", "Ratio of queries that should be traced.").Default("0.01").Float()
	)
	app.Version(version.Version)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("xgql"))

	kingpin.FatalIfError(otelruntime.Start(), "cannot add OpenTelemetry runtime instrumentation")
	res := resource.NewWithAttributes(attribute.String("service.name", "crossplane.io/gql"))

	// OpenTelemetry metrics.
	prom, err := prometheus.InstallNewPipeline(prometheus.Config{}, basic.WithResource(res))
	kingpin.FatalIfError(err, "cannot create OpenTelemetry Prometheus exporter")

	// OpenTelemetry tracing.
	if *agent != "" {
		flush, err := jaeger.InstallNewPipeline(jaeger.WithAgentEndpoint(*agent), jaeger.WithSDKOptions(
			trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(*ratio))),
			trace.WithResource(res)))
		kingpin.FatalIfError(err, "cannot create OpenTelemetry Jaeger exporter")
		defer flush()
	}

	// NOTE(negz): This handler is called when a cache can't watch a type that
	// it would like to, for example because the user doesn't have RBAC access
	// to watch that type, or because it was defined by a CRD that is now gone.
	// Ideally we'd terminate any cache in this state, but controller-runtime
	// does not surface the configurable watch error handling of the underlying
	// client-go machinery, so instead we just log it. The errors will persist
	// until they are resolved (e.g. the user is granted the RBAC access they
	// need) or the cache expires.
	utilruntime.ErrorHandlers = []func(error){func(err error) { log.Debug("Kubernetes runtime error", "err", err) }}

	rt := chi.NewRouter()
	rt.Use(middleware.RequestLogger(&formatter{log}))
	rt.Use(auth.Middleware)
	rt.Use(version.Middleware)

	s := runtime.NewScheme()
	kingpin.FatalIfError(corev1.AddToScheme(s), "cannot add Kubernetes core/v1 to scheme")
	kingpin.FatalIfError(kextv1.AddToScheme(s), "cannot add Kubernetes apiextensions/v1 to scheme")
	kingpin.FatalIfError(pkgv1.AddToScheme(s), "cannot add Crossplane pkg/v1 to scheme")
	kingpin.FatalIfError(extv1.AddToScheme(s), "cannot add Crossplane apiextensions/v1 to scheme")
	kingpin.FatalIfError(appsv1.AddToScheme(s), "cannot add Kubernetes apps/v1 to scheme")
	kingpin.FatalIfError(rbacv1.AddToScheme(s), "cannot add Kubernetes rbac/v1 to scheme")

	cfg, err := clients.Config()
	kingpin.FatalIfError(err, "cannot create client config")

	// We create a global REST mapper once here with _our_ credentials (not the
	// bearer token of each caller) because doing so is very slow; it can take
	// 10-15 seconds. Kubernetes allows any authenticated user to access the
	// discovery API via the system:discovery ClusterRoleBinding.
	t := time.Now()
	rm, err := apiutil.NewDynamicRESTMapper(cfg)
	kingpin.FatalIfError(err, "cannot create REST mapper")
	log.Debug("Created REST mapper", "duration", time.Since(t))

	ca := clients.NewCache(s,
		clients.Anonymize(cfg),
		clients.WithRESTMapper(rm),
		clients.DoNotCache(noCache),
		clients.WithLogger(log),
	)
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolvers.New(ca)}))
	srv.SetErrorPresenter(present.Error)
	srv.Use(opentelemetry.MetricEmitter{})
	srv.Use(opentelemetry.Tracer{})
	srv.Use(apollotracing.Tracer{})

	rt.Handle("/query", otelhttp.NewHandler(srv, "/query"))
	rt.Handle("/metrics", prom)
	rt.Handle("/version", version.Handler())
	if *play {
		rt.Handle("/", playground.Handler("GraphQL playground", "/query"))
	}

	h := http.Server{
		Handler:      rt,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  5 * time.Second,
		ErrorLog:     stdlog.New(ioutil.Discard, "", 0),
	}

	if *tlsCert != "" && *tlsKey != "" {
		go func() {
			log.Debug("Listening for TLS connections", "address", *listen)
			h.Addr = *listen
			kingpin.FatalIfError(h.ListenAndServeTLS(*tlsCert, *tlsKey), "cannot serve TLS HTTP")
		}()
	}

	log.Debug("Listening for insecure connections", "address", *insecure)
	h.Addr = *insecure
	kingpin.FatalIfError(h.ListenAndServe(), "cannot serve insecure HTTP")
}

type formatter struct{ log logging.Logger }

func (f *formatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &entry{log: f.log.WithValues(
		"id", middleware.GetReqID(r.Context()),
		"method", r.Method,
		"tls", r.TLS != nil,
		"host", r.Host,
		"uri", r.RequestURI,
		"protocol", r.Proto,
		"remote", r.RemoteAddr,
	)}
}

type entry struct{ log logging.Logger }

func (e *entry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	e.log.Debug("Handled request",
		"status", status,
		"bytes", bytes,
		"duration", elapsed,
	)
}

func (e *entry) Panic(v interface{}, stack []byte) {
	e.log.Debug("Paniced while handling request", "stack", stack, "panic", v)
}
