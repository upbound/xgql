package model

import (
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
)

// raw returns the supplied object as a JSON string. It panics if the object
// cannot be marshalled as JSON, which _should_ only happen if this program is
// fundamentally broken - e.g. trying to use a weird runtime.Object.
func raw(obj runtime.Object) string {
	out, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(out)
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
	if i == nil {
		return nil
	}

	out := int(*i)
	return &out
}
