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

package unstructured

import (
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	apiVersion = "apiextensions.k8s.io/v1"
	kind       = "CustomResourceDefinition"
	kindList   = "CustomResourceDefinitionList"
)

// CustomResourceDefinition wraps an underlying unstructured.Unstructured. We
// elect to use this type as a mechanism for working with CRDs versus the core
// CustomResourceDefinition type as a means to side step expensive
// un/marhsaling.
type CustomResourceDefinition struct {
	unstructured.Unstructured
}

// CustomResourceDefinitionList wraps an underlying
// unstructured.UnstructuredList. We elect to use this type as a mechanism for
// working with CRDs versus the core CustomResourceDefinition type as a means
// to side step expensive un/marhsaling.
type CustomResourceDefinitionList struct {
	unstructured.UnstructuredList
}

// NewCRD constructs a CustomResourceDefinition with APIVersion and Kind set.
func NewCRD() *CustomResourceDefinition {
	crd := &CustomResourceDefinition{}
	crd.SetAPIVersion(apiVersion)
	crd.SetKind(kind)

	return crd
}

// NewCRDList constructs a CustomResourceDefinitionList with APIVersion and Kind set.
func NewCRDList() *CustomResourceDefinitionList {
	list := &CustomResourceDefinitionList{}
	list.SetAPIVersion(apiVersion)
	list.SetKind(kindList)

	return list
}

// GetAPIVersion of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetAPIVersion() string {
	return apiVersion
}

// GetKind of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetKind() string {
	return kind
}

// GetUnstructured of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetUnstructuredList of this CustomResourceDefinitionList.
func (c *CustomResourceDefinitionList) GetUnstructuredList() *unstructured.UnstructuredList {
	return &c.UnstructuredList
}

// GetSpecGroup of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetSpecGroup() string {
	p, err := fieldpath.Pave(c.Object).GetString("spec.group")
	if err != nil {
		return ""
	}
	return p
}

// SetSpecGroup of this CustomResourceDefinition.
func (c *CustomResourceDefinition) SetSpecGroup(g string) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.group", g)
}

// GetSpecNames of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetSpecNames() kextv1.CustomResourceDefinitionNames {
	out := kextv1.CustomResourceDefinitionNames{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.names", &out); err != nil {
		return kextv1.CustomResourceDefinitionNames{}
	}
	return out
}

// SetSpecNames of this CustomResourceDefinition.
func (c *CustomResourceDefinition) SetSpecNames(n kextv1.CustomResourceDefinitionNames) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.names", n)
}

// GetSpecScope of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetSpecScope() kextv1.ResourceScope {
	p, err := fieldpath.Pave(c.Object).GetString("spec.scope")
	if err != nil {
		return ""
	}
	return kextv1.ResourceScope(p)
}

// SetSpecScope of this CustomResourceDefinition.
func (c *CustomResourceDefinition) SetSpecScope(s kextv1.ResourceScope) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.scope", s)
}

// GetStatus of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetStatus() kextv1.CustomResourceDefinitionStatus {
	out := kextv1.CustomResourceDefinitionStatus{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.status", &out); err != nil {
		return kextv1.CustomResourceDefinitionStatus{}
	}
	return out
}

// SetStatus of this CustomResourceDefinition.
func (c *CustomResourceDefinition) SetStatus(s kextv1.CustomResourceDefinitionStatus) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.status", s)
}

// GetSpecVersions of this CustomResourceDefinition.
func (c *CustomResourceDefinition) GetSpecVersions() []kextv1.CustomResourceDefinitionVersion {
	out := []kextv1.CustomResourceDefinitionVersion{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.versions", &out); err != nil {
		return []kextv1.CustomResourceDefinitionVersion{}
	}
	return out
}

// SetSpecVersions of this CustomResourceDefinition.
func (c *CustomResourceDefinition) SetSpecVersions(v []kextv1.CustomResourceDefinitionVersion) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.versions", v)
}
