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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
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

// JSONPatchReporter is a simple custom reporter that records the operations needed to
// transform one value into another.
type JSONPatchReporter struct {
	path  cmp.Path
	patch []Operation

	moved map[string]string
}

// PushStep implements cmp.Reporter.
func (r *JSONPatchReporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

// Report implements cmp.Reporter.
// we add any unequal operations to the list of operations.
func (r *JSONPatchReporter) Report(rs cmp.Result) {
	if rs.Equal() {
		return
	}
	assert(len(r.path) > 0)
	ps := r.path.Last()
	vx, vy := ps.Values()
	switch s := ps.(type) {
	case cmp.MapIndex, cmp.TypeAssertion:
		switch {
		// value did not exist.
		case !vx.IsValid():
			r.op(Add, r.toJSONPointer(r.path), vy.Interface(), "")
		// value does not exist.
		case !vy.IsValid():
			r.op(Remove, r.toJSONPointer(r.path), nil, "")
		// value replaced.
		default:
			r.op(Replace, r.toJSONPointer(r.path), vy.Interface(), "")
		}
	case cmp.SliceIndex:
		kx, ky := s.SplitKeys()
		switch {
		// value was updated.
		case kx == ky:
			r.op(Replace, r.toJSONPointer(r.path), vy.Interface(), "")
		// value did not exist before.
		case kx == -1:
			r.op(Add, r.toJSONPointer(r.path[:len(r.path)-1])+"/"+strconv.Itoa(ky), vy.Interface(), "")
		// value was removed.
		case ky == -1:
			r.op(Remove, r.toJSONPointer(r.path[:len(r.path)-1])+"/"+strconv.Itoa(kx), nil, "")
		// value was moved.
		default:
			r.move(
				r.toJSONPointer(r.path[:len(r.path)-1])+"/"+strconv.Itoa(kx),
				r.toJSONPointer(r.path[:len(r.path)-1])+"/"+strconv.Itoa(kx),
			)
		}
	default:
		panic(fmt.Sprintf("unknown path step type %T", s))
	}
}

// PopStep implements cmp.Reporter.
func (r *JSONPatchReporter) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

func (r *JSONPatchReporter) GetPatch() []Operation {
	return r.patch
}

var jsonPointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func (r *JSONPatchReporter) op(op Op, path string, value any, from string) {
	r.patch = append(r.patch, Operation{op, path, from, value})
}

func (r *JSONPatchReporter) move(from, to string) {
	if r.moved == nil {
		r.moved = make(map[string]string)
	}
	if prev, moved := r.moved[from]; moved {
		assert(prev == to)
	} else {
		r.moved[from] = to
		// record move patch
		r.op(Move, from, nil, to)
	}
}

func (r *JSONPatchReporter) toJSONPointer(path cmp.Path) string {
	var sb strings.Builder
	for _, ps := range path {
		switch s := ps.(type) {
		case cmp.MapIndex:
			// json must have string keys.
			assert(s.Key().Kind() == reflect.String)
			sb.WriteString("/")
			sb.WriteString(jsonPointerEscaper.Replace(s.Key().String()))
		case cmp.SliceIndex:
			if s.Key() < 0 {
				kx, ky := s.SplitKeys()
				// insertions and deletions are handled above
				assert(kx > 0 && ky > 0)
				from := sb.String() + "/" + strconv.Itoa(kx)
				sb.WriteString("/")
				sb.WriteString(strconv.Itoa(ky))
				r.move(from, sb.String())
				continue
			}
			// split keys for slices must be handled at a higher level.
			sb.WriteString("/")
			sb.WriteString(strconv.Itoa(s.Key()))
		}
	}
	return sb.String()
}

func assert(ok bool) {
	if !ok {
		panic("assertion failure")
	}
}

func parseJSON(in []byte) (out map[string]any) {
	if err := json.Unmarshal(in, &out); err != nil {
		panic(err) // should never occur given previous filter to ensure valid JSON
	}
	return out
}

// CreateJSONPatch creates a JSON patch between two json values.
func CreateJSONPatch(x, y []byte) ([]Operation, error) {
	if !json.Valid(x) || !json.Valid(y) {
		return nil, fmt.Errorf("invalid JSON")
	}
	r := &JSONPatchReporter{}
	if cmp.Equal(parseJSON(x), parseJSON(y), cmp.Reporter(r)) {
		return nil, nil
	}
	return r.GetPatch(), nil
}
