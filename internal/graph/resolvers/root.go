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

package resolvers

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
)

// Default resolver timeout.
const timeout = 5 * time.Second

// A ClientCache can produce a client for a given token.
type ClientCache interface {
	// Get a client for the given token.
	Get(cr auth.Credentials, o ...clients.GetOption) (client.Client, error)
}

// A ClientCacheFn is a function that can produce a client for a given token.
type ClientCacheFn func(cr auth.Credentials, o ...clients.GetOption) (client.Client, error)

// Get a client for the given token.
func (fn ClientCacheFn) Get(cr auth.Credentials, o ...clients.GetOption) (client.Client, error) {
	return fn(cr, o...)
}

// The Root resolver.
type Root struct {
	clients ClientCache
}

// New returns a new root resolver.
func New(cc ClientCache) *Root {
	return &Root{clients: cc}
}

// Query resolves GraphQL queries.
func (r *Root) Query() generated.QueryResolver {
	return &query{clients: r.clients}
}

// Mutation resolves GraphQL mutations.
func (r *Root) Mutation() generated.MutationResolver {
	return &mutation{clients: r.clients}
}

// ObjectMeta resolves properties of the ObjectMeta GraphQL type.
func (r *Root) ObjectMeta() generated.ObjectMetaResolver {
	return &objectMeta{clients: r.clients}
}

// Secret resolves properties of the Secret GraphQL type.
func (r *Root) Secret() generated.SecretResolver {
	return &secret{clients: r.clients}
}

// ConfigMap resolves properties of the ConfigMap GraphQL type.
func (r *Root) ConfigMap() generated.ConfigMapResolver {
	return &configMap{clients: r.clients}
}

// CompositeResource resolves properties of the CompositeResource GraphQL type.
func (r *Root) CompositeResource() generated.CompositeResourceResolver {
	return &compositeResource{clients: r.clients}
}

// CompositeResourceSpec resolves properties of the CompositeResourceSpec
// GraphQL type.
func (r *Root) CompositeResourceSpec() generated.CompositeResourceSpecResolver {
	return &compositeResourceSpec{clients: r.clients}
}

// CompositeResourceClaim resolves properties of the CompositeResourceClaim
// GraphQL type.
func (r *Root) CompositeResourceClaim() generated.CompositeResourceClaimResolver {
	return &compositeResourceClaim{clients: r.clients}
}

// CompositeResourceClaimSpec resolves properties of the CompositeResourceClaimSpec
// GraphQL type.
func (r *Root) CompositeResourceClaimSpec() generated.CompositeResourceClaimSpecResolver {
	return &compositeResourceClaimSpec{clients: r.clients}
}

// CompositeResourceDefinition resolves properties of the
// CompositeResourceDefinition GraphQL type.
func (r *Root) CompositeResourceDefinition() generated.CompositeResourceDefinitionResolver {
	return &xrd{clients: r.clients}
}

// CompositeResourceDefinitionSpec resolves properties of the
// CompositeResourceDefinitionSpec GraphQL type.
func (r *Root) CompositeResourceDefinitionSpec() generated.CompositeResourceDefinitionSpecResolver {
	return &xrdSpec{clients: r.clients}
}

// Composition resolves properties of the Composition GraphQL type.
func (r *Root) Composition() generated.CompositionResolver {
	return &composition{clients: r.clients}
}

// Configuration resolves properties of the Configuration GraphQL type.
func (r *Root) Configuration() generated.ConfigurationResolver {
	return &configuration{clients: r.clients}
}

// ConfigurationRevision resolves properties of the ConfigurationRevision
// GraphQL type.
func (r *Root) ConfigurationRevision() generated.ConfigurationRevisionResolver {
	return &configurationRevision{clients: r.clients}
}

// ConfigurationRevisionStatus resolves properties of the
// ConfigurationRevisionStatus GraphQL type.
func (r *Root) ConfigurationRevisionStatus() generated.ConfigurationRevisionStatusResolver {
	return &configurationRevisionStatus{clients: r.clients}
}

// CustomResourceDefinition resolves properties of the CustomResourceDefinition
// GraphQL type.
func (r *Root) CustomResourceDefinition() generated.CustomResourceDefinitionResolver {
	return &crd{clients: r.clients}
}

// Event resolves properties of the Event GraphQL type.
func (r *Root) Event() generated.EventResolver {
	return &event{clients: r.clients}
}

// GenericResource resolves properties of the GenericResource GraphQL type.
func (r *Root) GenericResource() generated.GenericResourceResolver {
	return &genericResource{clients: r.clients}
}

// ManagedResource resolves properties of the CustomResourceDefinition GraphQL
// type.
func (r *Root) ManagedResource() generated.ManagedResourceResolver {
	return &managedResource{clients: r.clients}
}

// ManagedResourceSpec resolves properties of the CustomResourceDefinition GraphQL
// type.
func (r *Root) ManagedResourceSpec() generated.ManagedResourceSpecResolver {
	return &managedResourceSpec{clients: r.clients}
}

// Provider resolves properties of the Provider GraphQL type.
func (r *Root) Provider() generated.ProviderResolver {
	return &provider{clients: r.clients}
}

// ProviderRevision resolves properties of the ProviderRevision GraphQL type.
func (r *Root) ProviderRevision() generated.ProviderRevisionResolver {
	return &providerRevision{clients: r.clients}
}

// ProviderRevisionStatus resolves properties of the ProviderRevisionStatus
// GraphQL type.
func (r *Root) ProviderRevisionStatus() generated.ProviderRevisionStatusResolver {
	return &providerRevisionStatus{clients: r.clients}
}

// ProviderConfig resolves properties of the ProviderConfig GraphQL type.
func (r *Root) ProviderConfig() generated.ProviderConfigResolver {
	return &providerConfig{clients: r.clients}
}
