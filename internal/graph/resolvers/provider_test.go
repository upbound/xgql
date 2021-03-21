package resolvers

import "github.com/negz/xgql/internal/graph/generated"

var (
	_ generated.ProviderResolver               = &provider{}
	_ generated.ProviderRevisionResolver       = &providerRevision{}
	_ generated.ProviderRevisionStatusResolver = &providerRevisionStatus{}
)
