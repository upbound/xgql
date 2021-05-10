// +build generate

// NOTE(negz): See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Generate xgql models, bindings, etc per gqlgen.yaml.
//go:generate go run -tags generate github.com/99designs/gqlgen

// Add license headers to all files.
//go:generate go run -tags generate github.com/google/addlicense -v -c "Upbound Inc" . ../cmd

package internal

import (
	_ "github.com/99designs/gqlgen"  //nolint:typecheck
	_ "github.com/google/addlicense" //nolint:typecheck
)
