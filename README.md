# xgql
An experimental @crossplane GraphQL API.

Crossplane is built atop the Kubernetes API, and makes heavy use of types that
may added and removed at runtime; for example installing a `Provider` adds many
new types of managed resource. A typical GraphQL query like "get me all the
managed resources that belong to this provider" can require _many_ REST API
calls. `xgql` uses [controller-runtime] client and cache machinery in order to
provide quick responses where possible.

Each GraphQL caller is expected to supply a valid Kubernetes API token via an
`Authorization` HTTP header formatted as `Bearer <token>` - the same format used
to access the Kubernetes REST API directly. `xgql` creates a unique REST client 
for each bearer token. Each REST client is rate limited to 5 requests per second
with a 10 request per second burst, and backed by an in-memory cache. Any time a
client gets or lists a particular type of resource it will begin automatically
caching that type of resource; the cache machinery takes a watch on the relevant
type to ensure the cache is always up-to-date. Each client and their cache is
garbage collected after 5 minutes of inactivity.

In an unscientific test of Crossplane 1.1 installed on a `kind` cluster running
on a GCP VM, with three providers installed, 118 CRDs, and a small handful of
configurations, XRDs, Compositions, XRs, provider configs, and managed resources
it takes:

* About 500ms to list (and count) all of the XRDs, Compositions, and XRs related
  to each installed configuration with a cold cache, and under 10ms with a warm
  cache.
* About 600ms to list (and count) all of the managed resources related to each
  installed provider with a cold cache, and about 50ms with a warm(ish) cache.
  Note that in this case the cache is warm-ish because we fetch the connection
  secret for each managed resource, and we don't cache secrets.

## Developing

Much of the GraphQL plumbing is built with [gqlgen], which is somewhat magic. In
particular `internal/graph/generated` is completely generated machinery. Models
in `internal/graph/model/generated.go` are generated from `schema/*.gql` -
gqlgen magically matches types in the `model` package by name and won't generate
them if they already exist. Generation of resolver stubs is disabled because it
is somewhat confusing and of little benefit.

To build:
```console
# Running a bare 'make' may be required to pull down the build submodule.
make

# Run code generation, linting, tests, and build artifacts.
make reviewable test build
```

## Example queries
Querying configurations:

```gql
{
  configurations {
    count
    items {
      metadata {
        name
        }
      revisions {
        count
        items {
          metadata {
            name
          }
          status {
            objects {
              count
              items {
                apiVersion
                kind
                metadata {
                  name
                }
                ...on Composition {
                  spec {
                    compositeTypeRef {
                      apiVersion
                      kind
                    }
                  }
                }
                ...on CompositeResourceDefinition {
                  definedCompositeResources {
                    count
                    items {
                      apiVersion
                      kind
                      metadata {
                        name
                      }
                      spec {
                        compositionSelector {
                          matchLabels
                        }
                      }
                      status {
                        conditions {
                          type
                          status
                          reason
                          lastTransitionTime
                          message
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
```

```gql
{
  providers {
    items {
      metadata {
        name
        }
        revisions {
        count
        items {
          metadata {
            name
          }
          status {
            objects {
              count
              items {
                apiVersion
                kind
                metadata {
                  name
                }
                ...on CustomResourceDefinition {
                  spec {
                    group
                    names {
                      kind
                    }
                  }
                  definedResources {
                    count
                    items {
                      ...on ManagedResource {
                        apiVersion
                          kind
                          metadata {
                            name
                          }
                          spec {
                          connectionSecret {
                            apiVersion
                            kind
                            metadata {
                              name
                            }
                          }
                        }         
                      }

                    }
                  }
                }
              }
            }
          }
        }
      }
      }
  }
}
```



[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[gqlgen]: https://github.com/99designs/gqlgen