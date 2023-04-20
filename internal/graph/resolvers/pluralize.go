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

package resolvers

import (
	pluralize "github.com/gertd/go-pluralize"
)

var (
	// NOTE(tnthornton) in order to "pluralize" a string, we need to
	// instantiate a new client. Instead of making that client a member of
	// a few pre-existing structs, we elect to instantiate globally and
	// provide a helper method of producing the plural form of the given
	// string.
	p = pluralize.NewClient()
)

// pluralForm returns the pluralized form of the given string.
func pluralForm(s string) string {
	return p.Plural(s)
}
