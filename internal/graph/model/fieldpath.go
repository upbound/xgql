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

import (
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// SkipUnstructured is a marker type. It is used in the schema via a `@goType` directive
// to change all "unstructured" fields to this type and make them embedded. This prevents
// gqlgen from generating resolver logic for these fields and instead makes it delegate
// resolutions to PavedAccess, which is also embedded via the `@goType` directive.
type SkipUnstructured interface{}

// PavedAccess is an embedded resolver for "unstructured" and "fieldPath" fields.
// It is embedded in generated types via a `@goType` directive.
type PavedAccess struct {
	*fieldpath.Paved
}

// Unstructured implements the "unstructured" field and returns the JSON representation
// of the entire object.
func (f PavedAccess) Unstructured() []byte {
	return raw(f.Paved)
}

// FieldPath implements the "fieldPath" field and returns the JSON representation
// of the value at the given path.
func (f PavedAccess) FieldPath(path *string) ([]byte, error) {
	if path == nil || len(*path) == 0 {
		return f.Unstructured(), nil
	}
	return fieldPath(f.Paved, *path)
}

// raw returns the supplied object as unstructured JSON bytes. It panics if
// the object cannot be marshalled as JSON, which _should_ only happen if this
// program is fundamentally broken - e.g. trying to use a weird runtime.Object.
func raw(paved *fieldpath.Paved) []byte {
	out, err := json.Marshal(paved)
	if err != nil {
		panic(err)
	}
	return out
}

// fieldPathWildcard expands the wildcard field path and returns the values at
// the resulting field paths for the supplied object as JSON bytes. It panics if
// the object cannot be marshalled as JSON, which _should_ only happen if this
// program is fundamentally broken.
func fieldPathWildcard(paved *fieldpath.Paved, path string) ([]byte, error) {
	arrayFieldPaths, err := paved.ExpandWildcards(path)
	if err != nil {
		return nil, err
	}

	if len(arrayFieldPaths) == 0 {
		return nil, errors.Errorf("cannot expand path %q", path)
	}

	array := make([]interface{}, 0, len(arrayFieldPaths))
	for _, arrayFieldPath := range arrayFieldPaths {
		v, err := paved.GetValue(arrayFieldPath)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get value at path %q", arrayFieldPath)
		}
		array = append(array, v)
	}
	out, err := json.Marshal(array)
	if err != nil {
		panic(err)
	}
	return out, nil
}

// fieldPath returns the value at the supplied field path for the supplied
// object as JSON bytes. It panics if the object cannot be marshalled as JSON,
// which _should_ only happen if this program is fundamentally broken.
func fieldPath(paved *fieldpath.Paved, path string) ([]byte, error) {
	if strings.Contains(path, "[*]") {
		return fieldPathWildcard(paved, path)
	}
	v, err := paved.GetValue(path)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out, nil
}

func paveObject(o runtime.Object) *fieldpath.Paved {
	p, err := fieldpath.PaveObject(o)
	if err != nil {
		panic(err)
	}
	return p
}
