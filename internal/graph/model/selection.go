// Copyright 2023 Upbound Inc
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

package model

// SelectedFields allows checking if a field was selected in a GraphQL query.
// It uses "." as a separator for nested fields.
// Example:
//
//	fields := GetSelectedFields(ctx)
//	if fields.Has("user") {
//	  // get base fields
//	}
//	if fields.Has("user.friends") {
//	  // do some expensive operation
//	}
type SelectedFields interface {
	Has(path string) bool
}

type selectFields bool

func (s selectFields) Has(string) bool {
	return bool(s)
}

var (
	SelectAll  selectFields = true
	SelectNone selectFields = false
)

type skipFields []string

func (s skipFields) Has(path string) bool {
	for _, f := range s {
		if f == path {
			return false
		}
		if len(f) > len(path) && f[:len(path)] == path && f[len(path)] == '.' {
			return false
		}
	}
	return true
}

func SkipFields(paths ...string) SelectedFields {
	return skipFields(paths)
}
