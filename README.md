# xgql [![CI](https://github.com/upbound/xgql/actions/workflows/ci.yml/badge.svg)](https://github.com/upbound/xgql/actions/workflows/ci.yml) [![Codecov](https://codecov.io/gh/upbound/xgql/branch/main/graph/badge.svg?token=QCRHAiqe1T)](https://codecov.io/gh/upbound/xgql)

A GraphQL API for [Crossplane].

![A screenshot of an xgql query in the GraphQL Playground](/images/playground.png)

Crossplane is built atop the Kubernetes API, and makes heavy use of types that
may added and removed at runtime; for example installing a `Provider` adds many
new types of managed resource. A typical GraphQL query like "get me all the
managed resources that belong to this provider" can require _many_ REST API
calls. `xgql` uses [controller-runtime] client and cache machinery in order to
provide quick responses where possible.

Each GraphQL caller is expected to supply valid Kubernetes API credentials via
an `Authorization` HTTP header containing either a [bearer token] or basic auth
credentials. [Impersonation headers] may also be included if the subject of the
`Authorization` header has been granted RBAC access to impersonate the subject
of the impersonation headers.

`xgql` creates a unique Kubernetes API client for each unique set of credentials
(including impersonation details). Each client is rate limited to 5 requests per
second with a 10 request per second burst, and backed by an in-memory cache. Any
time a client gets or lists a particular type of resource it will automatically
begin caching that type of resource; the cache machinery takes a watch on the
relevant type to ensure the cache is always up-to-date. Each client and their
cache is garbage collected after 5 minutes of inactivity.

Unscientific tests indicate that xgql's caches reduce GraphQL query times by an
order of magnitude; for example a query that takes ~500ms with a cold cache
takes 50ms or less with a warm cache.

## Developing

Much of the GraphQL plumbing is built with [gqlgen], which is somewhat magic. In
particular `internal/graph/generated` is completely generated machinery. Models
in `internal/graph/model/generated.go` are generated from `schema/*.gql` -
gqlgen magically matches types in the `model` package by name and won't generate
them if they already exist. Generation of resolver stubs is disabled because it
is somewhat confusing and of little benefit.

To try it out:

```console
# Running a bare 'make' may be required to pull down the build submodule.
make

# Lint, test, and build xgql
make reviewable test build

# Spin up a kind cluster.
./cluster/local/kind.sh up

# Install xgql
./cluster/local/kind.sh helm-install

# Install the latest Crossplane release (using Helm 3)
kubectl create namespace crossplane-system
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm install crossplane --namespace crossplane-system crossplane-stable/crossplane

# Install the Crossplane CLI - be sure to follow the instructions.
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | sh

# Install a Crossplane configuration.
# See https://crossplane.io/docs for the latest getting started configs.
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-gcp:v1.1.0

# Forward a local port
kubectl -n crossplane-system port-forward deployment/xgql 8080

# Open the GraphQL playground at http://localhost:8080

# Create service account to make GraphQL queries
kubectl apply -f cluster/local/service-account.yaml

# Get the service account's token (requires jq)
SA_TOKEN=$(kubectl get -o json secret xgql-testing|jq -r '.data.token|@base64d')

# Paste this into the HTTP Headers popout in the lower right of the playground
echo "{\"Authorization\":\"Bearer ${SA_TOKEN}\"}"
```

You may want to avoid deploying `xqgl` via Helm while developing. Instead you
can spin up a `kind` cluster, install Crossplane and run `xgql` outside the
cluster by running`go run cmd/xgql/main.go --debug`. In this mode `xgql` will
attempt to find and authenticate to a cluster by reading your `~/.kube/config`
file. All authentication methods are stripped from the kubeconfig file so
GraphQL requests must still supply authz headers.

[crossplane]: https://crossplane.io
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[gqlgen]: https://github.com/99designs/gqlgen
[bearer token]: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#putting-a-bearer-token-in-a-request
[impersonation headers]: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation
