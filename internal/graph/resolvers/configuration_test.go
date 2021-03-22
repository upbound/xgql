package resolvers

import "github.com/upbound/xgql/internal/graph/generated"

var (
	_ generated.ConfigurationResolver               = &configuration{}
	_ generated.ConfigurationRevisionResolver       = &configurationRevision{}
	_ generated.ConfigurationRevisionStatusResolver = &configurationRevisionStatus{}
)
