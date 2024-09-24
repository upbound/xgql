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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/upbound/xgql/internal/unstructured"
)

func TestPavedAccess_FieldPath(t *testing.T) {
	transition := time.Now()
	crd := func() *unstructured.CustomResourceDefinition {
		crd := unstructured.NewCRD()
		crd.SetName("cool")
		crd.SetSpecGroup("group")
		crd.SetSpecNames(kextv1.CustomResourceDefinitionNames{
			Plural:     "clusterexamples",
			Singular:   "clusterexample",
			ShortNames: []string{"cex"},
			Kind:       "ClusterExample",
			ListKind:   "ClusterExampleList",
			Categories: []string{"example"},
		})
		crd.SetSpecScope(kextv1.NamespaceScoped)
		crd.SetSpecVersions([]kextv1.CustomResourceDefinitionVersion{{
			Name:   "v1",
			Served: true,
			Schema: &kextv1.CustomResourceValidation{
				OpenAPIV3Schema: &kextv1.JSONSchemaProps{},
			},
		}})
		crd.SetStatus(kextv1.CustomResourceDefinitionStatus{
			Conditions: []kextv1.CustomResourceDefinitionCondition{{
				Type:               kextv1.Established,
				Reason:             "VeryCoolCRD",
				Message:            "So cool",
				LastTransitionTime: metav1.NewTime(transition),
			}},
		})
		return crd
	}()
	// TODO(avalanche123): we can't reproduce exact errors from fieldpath.
	// the errNotFound is unexported and doesn't have a constructor.
	// Additionally, the type information from top level call to cmp.Diff
	// is not getting propagated correctly, so we use a FilterValue+Transformer
	// instead of a Comparer here. see https://github.com/google/go-cmp/issues/161.
	equateErrorMessage := func() cmp.Option {
		return cmp.FilterValues(func(x, y any) bool {
			_, ok1 := x.(error)
			_, ok2 := y.(error)
			return ok1 && ok2
		}, cmp.Transformer("T", func(e any) string {
			return e.(error).Error()
		}))
	}
	transformJSON := cmpopts.AcyclicTransformer("ToJSON", func(in any) []byte {
		if in == nil {
			return nil
		}
		if out, ok := in.([]byte); ok {
			if len(out) == 0 {
				return nil
			}
			return out
		}
		out, err := json.Marshal(in)
		if err != nil {
			panic(err)
		}
		return out
	})
	type args struct {
		fieldPath *string
		object    runtime.Object
	}
	type want struct {
		out any
		err error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Returns value at fieldPath.",
			args: args{
				fieldPath: ptr.To("metadata.name"),
				object:    crd,
			},
			want: want{
				out: "cool",
			},
		},
		"Wildcard": {
			reason: "Returns all values in expanded wildcard paths.",
			args: args{
				fieldPath: ptr.To("spec.status.conditions[*].reason"),
				object:    crd,
			},
			want: want{
				out: []string{"VeryCoolCRD"},
			},
		},
		"WrongPath": {
			reason: "Returns nil.",
			args: args{
				fieldPath: ptr.To("bad.field.path"),
				object:    crd,
			},
			want: want{
				out: []byte(nil),
				err: fmt.Errorf("%s: %s", "bad", "no such field"),
			},
		},
		"EmptyPath": {
			reason: "Returns the entire object.",
			args: args{
				object: crd,
			},
			want: want{
				out: map[string]any{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]string{
						"name": "cool",
					},
					"spec": map[string]any{
						"group": "group",
						"names": map[string]any{
							"plural":     "clusterexamples",
							"singular":   "clusterexample",
							"shortNames": []string{"cex"},
							"kind":       "ClusterExample",
							"listKind":   "ClusterExampleList",
							"categories": []string{"example"},
						},
						"scope": "Namespaced",
						"status": map[string]any{
							"acceptedNames": map[string]string{
								"kind":   "",
								"plural": "",
							},
							"conditions": []map[string]any{
								{
									"status":             "",
									"type":               kextv1.Established,
									"reason":             "VeryCoolCRD",
									"message":            "So cool",
									"lastTransitionTime": transition.UTC().Format(time.RFC3339),
								},
							},
							"storedVersions": nil,
						},
						"versions": []map[string]any{
							{
								"name": "v1",
								"schema": map[string]any{
									"openAPIV3Schema": map[string]any{},
								},
								"served":  true,
								"storage": false,
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			f := PavedAccess{
				Paved: paveObject(tc.args.object),
			}
			out, err := f.FieldPath(tc.args.fieldPath)
			if diff := cmp.Diff(tc.want.err, err, equateErrorMessage()); diff != "" {
				t.Errorf("\n%s\nPavedAccess.FieldPath(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.out, out, transformJSON); diff != "" {
				t.Errorf("\n%s\nPavedAccess.FieldPath(...): -want, +got:\n%s", tc.reason, diff)
				return
			}
		})
	}
}
