package resolvers

import "github.com/negz/xgql/internal/graph/generated"

var (
	_ generated.ConfigurationResolver               = &configuration{}
	_ generated.ConfigurationRevisionResolver       = &configurationRevision{}
	_ generated.ConfigurationRevisionStatusResolver = &configurationRevisionStatus{}
)
