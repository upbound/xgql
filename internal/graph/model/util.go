package model

import (
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
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
