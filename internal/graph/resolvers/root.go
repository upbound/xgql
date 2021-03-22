package resolvers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/negz/xgql/internal/graph/generated"
)

// A ClientCache can produce a client for a given token.
type ClientCache interface {
	Get(token string) (client.Client, error)
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

// ObjectMeta resolves properties of the ObjectMeta GraphQL type.
func (r *Root) ObjectMeta() generated.ObjectMetaResolver {
	return &objectMetaResolver{clients: r.clients}
}

// Secret resolves properties of the Secret GraphQL type.
func (r *Root) Secret() generated.SecretResolver {
	return nil
}

// CompositeResource resolves properties of the CompositeResource GraphQL type.
func (r *Root) CompositeResource() generated.CompositeResourceResolver {
	return nil
}

// CompositeResourceClaim resolves properties of the CompositeResourceClaim
// GraphQL type.
func (r *Root) CompositeResourceClaim() generated.CompositeResourceClaimResolver {
	return nil
}

// CompositeResourceClaimSpec resolves properties of the CompositeResourceClaimSpec
// GraphQL type.
func (r *Root) CompositeResourceClaimSpec() generated.CompositeResourceClaimSpecResolver {
	return nil
}

// CompositeResourceDefinition resolves properties of the
// CompositeResourceDefinition GraphQL type.
func (r *Root) CompositeResourceDefinition() generated.CompositeResourceDefinitionResolver {
	return nil
}

// CompositeResourceDefinitionSpec resolves properties of the
// CompositeResourceDefinitionSpec GraphQL type.
func (r *Root) CompositeResourceDefinitionSpec() generated.CompositeResourceDefinitionSpecResolver {
	return nil
}

// CompositeResourceSpec resolves properties of the CompositeResourceSpec
// GraphQL type.
func (r *Root) CompositeResourceSpec() generated.CompositeResourceSpecResolver {
	return nil
}

// Composition resolves properties of the Composition GraphQL type.
func (r *Root) Composition() generated.CompositionResolver {
	return nil
}

// Configuration resolves properties of the Configuration GraphQL type.
func (r *Root) Configuration() generated.ConfigurationResolver {
	return nil
}

// ConfigurationRevision resolves properties of the ConfigurationRevision
// GraphQL type.
func (r *Root) ConfigurationRevision() generated.ConfigurationRevisionResolver {
	return nil
}

// ConfigurationRevisionStatus resolves properties of the
// ConfigurationRevisionStatus GraphQL type.
func (r *Root) ConfigurationRevisionStatus() generated.ConfigurationRevisionStatusResolver {
	return nil
}

// CustomResourceDefinition resolves properties of the CustomResourceDefinition
// GraphQL type.
func (r *Root) CustomResourceDefinition() generated.CustomResourceDefinitionResolver {
	return &crd{clients: r.clients}
}

// Event resolves properties of the Event GraphQL type.
func (r *Root) Event() generated.EventResolver {
	return nil
}

// GenericResource resolves properties of the GenericResource GraphQL type.
func (r *Root) GenericResource() generated.GenericResourceResolver {
	return nil
}

// ManagedResource resolves properties of the CustomResourceDefinition GraphQL
// type.
func (r *Root) ManagedResource() generated.ManagedResourceResolver {
	return nil
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
	return nil
}
