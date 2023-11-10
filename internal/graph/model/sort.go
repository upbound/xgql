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

func join(id ReferenceID) string {
	return id.APIVersion + id.Kind + id.Namespace + id.Name
}

type identifiable interface{ id() ReferenceID }

// All types that satisfy the KubernetesResource GraphQL interface must make
// their ID available for sorting.
func (r ManagedResource) id() ReferenceID             { return r.ID }
func (r ProviderConfig) id() ReferenceID              { return r.ID }
func (r CompositeResource) id() ReferenceID           { return r.ID }
func (r CompositeResourceClaim) id() ReferenceID      { return r.ID }
func (r Provider) id() ReferenceID                    { return r.ID }
func (r ProviderRevision) id() ReferenceID            { return r.ID }
func (r Configuration) id() ReferenceID               { return r.ID }
func (r ConfigurationRevision) id() ReferenceID       { return r.ID }
func (r CompositeResourceDefinition) id() ReferenceID { return r.ID }
func (r Composition) id() ReferenceID                 { return r.ID }
func (r CustomResourceDefinition) id() ReferenceID    { return r.ID }
func (r Secret) id() ReferenceID                      { return r.ID }
func (r ConfigMap) id() ReferenceID                   { return r.ID }
func (r GenericResource) id() ReferenceID             { return r.ID }

func (c *KubernetesResourceConnection) Len() int { return c.TotalCount }
func (c *KubernetesResourceConnection) Less(i, j int) bool {
	return join(c.Nodes[i].(identifiable).id()) < join(c.Nodes[j].(identifiable).id())
}
func (c *KubernetesResourceConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *EventConnection) Len() int { return c.TotalCount }
func (c *EventConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *EventConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *ProviderConnection) Len() int { return c.TotalCount }
func (c *ProviderConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *ProviderConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *ProviderRevisionConnection) Len() int { return c.TotalCount }
func (c *ProviderRevisionConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *ProviderRevisionConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *CustomResourceDefinitionConnection) Len() int { return c.TotalCount }
func (c *CustomResourceDefinitionConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *CustomResourceDefinitionConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *ConfigurationConnection) Len() int { return c.TotalCount }
func (c *ConfigurationConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *ConfigurationConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *ConfigurationRevisionConnection) Len() int { return c.TotalCount }
func (c *ConfigurationRevisionConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *ConfigurationRevisionConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *CompositeResourceDefinitionConnection) Len() int { return c.TotalCount }
func (c *CompositeResourceDefinitionConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *CompositeResourceDefinitionConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *CompositionConnection) Len() int { return c.TotalCount }
func (c *CompositionConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *CompositionConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *CompositeResourceConnection) Len() int { return c.TotalCount }
func (c *CompositeResourceConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *CompositeResourceConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}

func (c *CompositeResourceClaimConnection) Len() int { return c.TotalCount }
func (c *CompositeResourceClaimConnection) Less(i, j int) bool {
	return join(c.Nodes[i].ID) < join(c.Nodes[j].ID)
}
func (c *CompositeResourceClaimConnection) Swap(i, j int) {
	c.Nodes[i], c.Nodes[j] = c.Nodes[j], c.Nodes[i]
}
