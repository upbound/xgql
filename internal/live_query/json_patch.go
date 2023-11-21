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

package live_query

import (
	"encoding/json"

	jd "github.com/josephburnett/jd/lib"
)

// Op is a JSON Patch operation.
type Op string

const (
	// Add contains "path" and "value" members.
	Add Op = "add"
	// Remove contains "path" member.
	Remove Op = "remove"
	// Replace contains "path" and "value" members.
	Replace Op = "replace"
	// Move contains "path" and "from" members.
	Move Op = "move"
)

// Operation is a single JSON Patch operation.
type Operation struct {
	Op    Op     `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}

// CreateJSONPatch creates a JSON patch between two json values.
// TODO(avalanche123): add tests for json patch generation.
func CreateJSONPatch(x, y jd.JsonNode) ([]Operation, error) {
	raw, err := x.Diff(y).RenderPatch()
	if err != nil {
		return nil, err
	}
	var patch []Operation
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		return nil, err
	}
	for i := 1; i < len(patch); i++ {
		// previous operation and operation
		rm, ad := patch[i-1], patch[i]
		if rm.Path != ad.Path {
			continue
		}
		// coalesce remove and add into a replace
		if rm.Op == Remove && ad.Op == Add {
			patch[i] = Operation{
				Path:  ad.Path,
				Op:    Replace,
				Value: ad.Value,
			}
			patch = append(patch[:i-1], patch[i:]...)
		}
	}
	return patch, nil
}
