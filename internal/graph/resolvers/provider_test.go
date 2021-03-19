package resolvers

import "github.com/negz/xgql/internal/graph/generated"

var (
	_ generated.ProviderResolver               = &provider{}
	_ generated.ProviderSpecResolver           = &providerSpec{}
	_ generated.ProviderRevisionResolver       = &providerRevision{}
	_ generated.ProviderRevisionSpecResolver   = &providerRevisionSpec{}
	_ generated.ProviderRevisionStatusResolver = &providerRevisionStatus{}
)
