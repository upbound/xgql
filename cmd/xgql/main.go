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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	// NOTE(tnthornton) we are making an active choice to have a pprof endpoint
	// available.
	_ "net/http/pprof" //nolint:gosec

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/apollotracing"
	gqldebug "github.com/99designs/gqlgen/graphql/handler/debug"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	google "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap/zapcore"
	"gopkg.in/alecthomas/kingpin.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal"
	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/cache"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/present"
	"github.com/upbound/xgql/internal/graph/resolvers"
	"github.com/upbound/xgql/internal/live_query"
	"github.com/upbound/xgql/internal/opentelemetry"
	"github.com/upbound/xgql/internal/request"
	hprobe "github.com/upbound/xgql/internal/server/health"
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
	&corev1.Node{},
	&corev1.Namespace{},
	&corev1.Pod{},
	&corev1.ConfigMap{},
	&corev1.Service{},
	&corev1.ServiceAccount{},
	&appsv1.Deployment{},
	&appsv1.DaemonSet{},
	&rbacv1.RoleBinding{},
	&rbacv1.ClusterRoleBinding{},
}

func main() { //nolint:gocyclo
	var (
		app             = kingpin.New(filepath.Base(os.Args[0]), "A GraphQL API for Crossplane.").DefaultEnvars()
		debug           = app.Flag("debug", "Enable debug logging.").Short('d').Counter()
		listen          = app.Flag("listen", "Address at which to listen for TLS connections. Requires TLS cert and key.").Default(":8443").String()
		tlsCert         = app.Flag("tls-cert", "Path to the TLS certificate file used to serve TLS connections.").ExistingFile()
		tlsKey          = app.Flag("tls-key", "Path to the TLS key file used to serve TLS connections.").ExistingFile()
		insecure        = app.Flag("listen-insecure", "Address at which to listen for insecure connections.").Default("127.0.0.1:8080").String()
		play            = app.Flag("enable-playground", "Serve a GraphQL Playground.").Bool()
		tracer          = app.Flag("trace-backend", "Tracer to use.").Default("jaeger").Enum("jaeger", "gcp", "stdout")
		ratio           = app.Flag("trace-ratio", "Ratio of queries that should be traced.").Default("0.01").Float()
		agent           = app.Flag("trace-agent", "Address of the Jaeger trace agent as [host]:[port]").TCP()
		health          = app.Flag("health", "Enable health endpoints.").Default("true").Bool()
		healthPort      = app.Flag("health-port", "Port used for readyz and livez requests.").Default("8088").Int()
		cacheExpiry     = app.Flag("cache-expiry", "The duration since last activity by a user until that users client expires.").Default("30m").Duration()
		profiling       = app.Flag("profiling", "Enable profiling via web interface host:port/debug/pprof/.").Default("true").Bool()
		cacheFile       = app.Flag("cache-file", "Path to the file used to persist client caches, set to reduce memory usage.").Default("").String()
		noApolloTracing = app.Flag("disable-apollo-tracing", "Disable apollo tracing.").Bool()

		globalEventsTarget = app.Flag("global-events-target", "The targeted number of events returned for global scope, potentially more if there are few warnings.").Default("500").Int()
		globalEventsCap    = app.Flag("global-events-cap", "The maximum number of events returned for global scope.").Default("2000").Int()
	)
	app.Version(version.Version)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	fs := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(fs)
	kingpin.FatalIfError(fs.Parse([]string{fmt.Sprintf("--v=%d", *debug)}), "cannot parse klog flags")

	zl := zap.New(zap.UseDevMode(*debug > 0))
	if *debug > 0 {
		klog.SetLogger(zap.New(zap.UseDevMode(*debug > 0)))
		ctrl.SetLogger(zap.New(zap.UseDevMode(*debug > 0)))
	} else {
		klog.SetLogger(zap.New(zap.Level(zapcore.ErrorLevel)))
		ctrl.SetLogger(zap.New(zap.Level(zapcore.ErrorLevel)))
	}
	log := logging.NewLogrLogger(zl.WithName("xgql"))

	// Start a pprof endpoint to ensure we can gather pprofs when needed.
	if *profiling {
		go func() {
			log.Info("pprof", "error", http.ListenAndServe("localhost:6060", nil)) //nolint:gosec
		}()
	}

	kingpin.FatalIfError(otelruntime.Start(), "cannot add OpenTelemetry runtime instrumentation")

	res := resource.NewSchemaless(attribute.String("service.name", "crossplane.io/xgql"))

	switch *tracer {
	case "jaeger":
		// We require the Jaeger agent address to be specified in order
		// to enable Jaeger for backward compatibility with older xgql
		// versions that only supported Jaeger.
		if *agent == nil {
			break
		}
		log.Debug("Enabling Jaeger tracer")
		exp, err := jaeger.New(jaeger.WithAgentEndpoint(jaeger.WithAgentHost((*agent).IP.String()), jaeger.WithAgentPort(strconv.Itoa((*agent).Port))))
		kingpin.FatalIfError(err, "cannot create OpenTelemetry Jaeger exporter")
		tp := trace.NewTracerProvider(trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(*ratio))), trace.WithResource(res), trace.WithBatcher(exp))
		defer func() {
			kingpin.FatalIfError(tp.Shutdown(context.Background()), "cannot shutdown Jaeger exporter")
		}()
		otel.SetTracerProvider(tp)
	case "gcp":
		log.Debug("Enabling GCP tracer")
		exp, err := google.New()
		kingpin.FatalIfError(err, "cannot create OpenTelemetry GCP exporter")
		tp := trace.NewTracerProvider(trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(*ratio))), trace.WithResource(res), trace.WithBatcher(exp))
		defer func() {
			kingpin.FatalIfError(tp.Shutdown(context.Background()), "cannot shutdown GCP exporter")
		}()
		otel.SetTracerProvider(tp)
	}

	// NOTE(negz): This handler is called when a cache can't watch a type that
	// it would like to, for example because the user doesn't have RBAC access
	// to watch that type, or because it was defined by a CRD that is now gone.
	// Ideally we'd terminate any cache in this state, but controller-runtime
	// does not surface the configurable watch error handling of the underlying
	// client-go machinery, so instead we just log it. The errors will persist
	// until they are resolved (e.g. the user is granted the RBAC access they
	// need) or the cache expires.
	//nolint:reassign
	utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
		func(ctx context.Context, err error, msg string, keysAndValues ...interface{}) {
			log.Debug("Kubernetes runtime error", "err", err)
		},
	}

	s := runtime.NewScheme()
	kingpin.FatalIfError(corev1.AddToScheme(s), "cannot add Kubernetes core/v1 to scheme")
	kingpin.FatalIfError(kextv1.AddToScheme(s), "cannot add Kubernetes apiextensions/v1 to scheme")
	kingpin.FatalIfError(pkgv1.AddToScheme(s), "cannot add Crossplane pkg/v1 to scheme")
	kingpin.FatalIfError(extv1.AddToScheme(s), "cannot add Crossplane apiextensions/v1 to scheme")
	kingpin.FatalIfError(appsv1.AddToScheme(s), "cannot add Kubernetes apps/v1 to scheme")
	kingpin.FatalIfError(rbacv1.AddToScheme(s), "cannot add Kubernetes rbac/v1 to scheme")

	cfg, err := clients.Config()
	kingpin.FatalIfError(err, "cannot create client config")

	httpClient, err := rest.HTTPClientFor(cfg)
	kingpin.FatalIfError(err, "cannot create HTTP client")

	// Our Kubernetes clients need to know what REST API resources are offered
	// by the API server. The discovery process takes a few ms and makes many
	// API server calls. Kubernetes allows any authenticated user to access the
	// discovery API via the system:discovery ClusterRoleBinding, so we create
	// a global REST mapper using our own credentials for all clients to share.
	// Discovery happens once at startup, and then once any time a client asks
	// for an unknown kind of API resource (subject to caching/rate limiting).
	rm, err := clients.RESTMapper(cfg, httpClient)
	kingpin.FatalIfError(err, "cannot create REST mapper")

	var camid []clients.NewCacheMiddlewareFn
	// wrap client.Cache in cache.*BBoltCache if cacheFile is specified.
	if *cacheFile != "" {
		camid = append(camid, cache.WithBBoltCache(*cacheFile, cache.WithLogger(log)))
	}
	// enable live queries
	camid = append(camid, cache.WithLiveQueries)

	caopts := []clients.CacheOption{
		clients.WithRESTMapper(rm),
		clients.DoNotCache(noCache),
		clients.WithLogger(log),
		clients.WithExpiry(*cacheExpiry),
		clients.UseNewCacheMiddleware(camid...),
	}
	ca := clients.NewCache(s, clients.Anonymize(cfg), caopts...)
	h := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolvers.New(ca)}))

	h.AddTransport(transport.Websocket{
		Upgrader: websocket.Upgrader{
			// Enable per message compression.
			EnableCompression: true,
		},
		PingPongInterval: 10 * time.Second,
		InitFunc:         auth.WebsocketInit,
	})
	h.AddTransport(transport.Options{})
	h.AddTransport(transport.GET{})
	h.AddTransport(transport.POST{})
	h.AddTransport(transport.MultipartForm{})

	h.SetQueryCache(lru.New(1000))

	h.Use(extension.Introspection{})
	h.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
	})

	h.SetErrorPresenter(present.Error)
	h.Use(opentelemetry.MetricEmitter{})
	h.Use(opentelemetry.Tracer{})
	if !*noApolloTracing {
		h.Use(apollotracing.Tracer{})
	}
	if *tracer == "stdout" {
		h.Use(&gqldebug.Tracer{})
	}
	h.Use(live_query.LiveQuery{})

	rt := chi.NewRouter()
	rt.Use(middleware.RequestID)
	// if bbolt cache is enabled, add up bolt transaction request middleware
	// to coalesce all concurrent reads from bolt db into a single transaction
	// in the context of a given request.
	if *cacheFile != "" {
		rt.Use(cache.BoltTxMiddleware)
	}
	rt.Use(middleware.RequestLogger(&request.Formatter{Log: log}))
	rt.Use(middleware.Compress(5)) // Chi recommends compression level 5.
	rt.Use(auth.Middleware)
	rt.Use(version.Middleware)
	rt.Use(resolvers.InjectConfig(&resolvers.Config{
		GlobalEventsTarget: *globalEventsTarget,
		GlobalEventsCap:    *globalEventsCap,
	}))

	rt.Handle("/query", otelhttp.NewHandler(h, "/query"))
	rt.Handle("/metrics", promhttp.Handler())
	rt.Handle("/version", version.Handler())
	if *play {
		rt.Handle("/", playground.Handler("GraphQL playground", "/query"))
	}

	// start health endpoints to aid in routing traffic to the pod
	kingpin.FatalIfError(startHealth(internal.HealthOptions{Health: *health, HealthPort: *healthPort}, log), "cannot start health endpoints")

	if *tlsCert != "" && *tlsKey != "" {
		srv := &http.Server{
			Addr:              *listen,
			Handler:           rt,
			WriteTimeout:      10 * time.Second,
			ReadTimeout:       5 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			ErrorLog:          stdlog.New(io.Discard, "", 0),
		}
		go func() {
			log.Debug("Listening for TLS connections", "address", *listen)
			kingpin.FatalIfError(srv.ListenAndServeTLS(*tlsCert, *tlsKey), "cannot serve TLS HTTP")
		}()
	}

	log.Debug("Listening for insecure connections", "address", *insecure)
	srv := &http.Server{
		Addr:              *insecure,
		Handler:           rt,
		WriteTimeout:      10 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          stdlog.New(io.Discard, "", 0),
	}
	kingpin.FatalIfError(srv.ListenAndServe(), "cannot serve insecure HTTP")
}

// startHealth starts the readyz and livez endpoints for this service.
func startHealth(opts internal.HealthOptions, log logging.Logger) error {
	p, err := hprobe.Server(opts, log)

	if err != nil {
		return err
	}

	go func() {
		log.Debug("Listening for Health connections", "address", fmt.Sprintf(":%d", opts.HealthPort))
		if err := p.ListenAndServe(); !errors.As(err, http.ErrServerClosed) {
			log.Info(errors.Wrap(err, "service stopped unexpectedly").Error())
			os.Exit(-1)
		}
	}()

	return nil
}
