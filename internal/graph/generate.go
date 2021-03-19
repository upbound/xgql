// +build generate

// NOTE(negz): See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

//go:generate go run -tags generate github.com/99designs/gqlgen

package graph

import (
	_ "github.com/99designs/gqlgen" //nolint:typecheck
)
