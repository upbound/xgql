// Copyright 2021 Upbound Inc
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
	"github.com/go-json-experiment/json"

	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// unstruct returns the supplied object as unstructured JSON bytes. It panics if
// the object cannot be marshalled as JSON, which _should_ only happen if this
// program is fundamentally broken - e.g. trying to use a weird runtime.Object.
func unstruct(obj runtime.Object) []byte {
	out, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return out
}

// bytesForUnstructured returns the supplied unstructured.Unstructured as JSON
// bytes. It panics if the object cannot be marshalled as JSON, which _should_
// only happen if this program is fundamentally broken.
func bytesForUnstructured(u *kunstructured.Unstructured) []byte {
	out, err := json.Marshal(u.Object)
	if err != nil {
		panic(err)
	}
	return out
}

func convert(from *kunstructured.Unstructured, to runtime.Object) error {
	c := runtime.DefaultUnstructuredConverter
	if err := c.FromUnstructured(from.Object, to); err != nil {
		return errors.Wrap(err, "could not convert unstructured object")
	}
	// For whatever reason the *Unstructured's GVK doesn't seem to make it
	// through the conversion process.
	gvk := schema.FromAPIVersionAndKind(from.GetAPIVersion(), from.GetKind())
	to.GetObjectKind().SetGroupVersionKind(gvk)
	return nil
}

func getIntPtr(i *int64) *int {
	if i == nil || *i == 0 {
		return nil
	}

	out := int(*i)
	return &out
}
