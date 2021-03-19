package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/alecthomas/kingpin.v2"

	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/negz/xgql/internal/clients"
	"github.com/negz/xgql/internal/graph/generated"
	"github.com/negz/xgql/internal/graph/resolvers"
	"github.com/negz/xgql/internal/token"
)

func main() {

	var (
		app    = kingpin.New(filepath.Base(os.Args[0]), "AWS support for Crossplane.").DefaultEnvars()
		listen = app.Flag("listen", "Address to listen at").Default(":8080").String()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	rt := chi.NewRouter()
	rt.Use(token.Middleware)
	rt.Use(middleware.Logger)

	s := runtime.NewScheme()
	kingpin.FatalIfError(corev1.AddToScheme(s), "cannot add Kubernetes core/v1 to scheme")
	kingpin.FatalIfError(kextv1.AddToScheme(s), "cannot add Kubernetes apiextensions/v1 to scheme")
	kingpin.FatalIfError(pkgv1.AddToScheme(s), "cannot add Crossplane pkg/v1 to scheme")
	kingpin.FatalIfError(extv1.AddToScheme(s), "cannot add Crossplane apiextensions/v1 to scheme")

	cfg, err := clients.AnonymousConfig()
	kingpin.FatalIfError(err, "cannot create client config")

	gc := generated.Config{Resolvers: resolvers.New(clients.NewCache(s, cfg))}
	rt.Handle("/query", handler.NewDefaultServer(generated.NewExecutableSchema(gc)))
	rt.Handle("/", playground.Handler("GraphQL playground", "/query"))

	kingpin.FatalIfError(http.ListenAndServe(*listen, rt), "cannot listen for HTTP")
}
