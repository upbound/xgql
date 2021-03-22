package model

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// A ProviderRevisionStatus reflects the observed state of a ProviderRevision.
type ProviderRevisionStatus struct {
	Conditions            []Condition  `json:"conditions"`
	FoundDependencies     *int         `json:"foundDependencies"`
	InstalledDependencies *int         `json:"installedDependencies"`
	InvalidDependencies   *int         `json:"invalidDependencies"`
	PermissionRequests    []PolicyRule `json:"permissionRequests"`

	ObjectRefs []xpv1.TypedReference
}

// IsConditionedStatus indicates that ProviderRevisionStatus satisfies the
// KubernetesResource GraphQL (and corresponding Go) interface.
func (ProviderRevisionStatus) IsConditionedStatus() {}

// A ConfigurationRevisionStatus reflects the observed state of a ConfigurationRevision.
type ConfigurationRevisionStatus struct {
	Conditions            []Condition  `json:"conditions"`
	FoundDependencies     *int         `json:"foundDependencies"`
	InstalledDependencies *int         `json:"installedDependencies"`
	InvalidDependencies   *int         `json:"invalidDependencies"`
	PermissionRequests    []PolicyRule `json:"permissionRequests"`

	ObjectRefs []xpv1.TypedReference
}

// IsConditionedStatus indicates that ConfigurationRevisionStatus satisfies the
// KubernetesResource GraphQL (and corresponding Go) interface.
func (ConfigurationRevisionStatus) IsConditionedStatus() {}

// GetRevisionActivationPolicy from the supplied Crossplane policy.
func GetRevisionActivationPolicy(in *pkgv1.RevisionActivationPolicy) *RevisionActivationPolicy {
	if in == nil {
		return nil
	}
	out := RevisionActivationPolicy(*in)
	return &out
}

// GetPackagePullPolicy from the supplied Kubernetes policy.
func GetPackagePullPolicy(in *corev1.PullPolicy) *PackagePullPolicy {
	if in == nil {
		return nil
	}
	out := PackagePullPolicy(*in)
	return &out
}

// GetPolicyRules from the supplied Kubernetes policy rules.
func GetPolicyRules(in []rbacv1.PolicyRule) []PolicyRule {
	out := make([]PolicyRule, len(in))
	for i := range in {
		out[i] = PolicyRule{
			Verbs:           in[i].Verbs,
			APIGroups:       in[i].APIGroups,
			Resources:       in[i].Resources,
			ResourceNames:   in[i].ResourceNames,
			NonResourceURLs: in[i].NonResourceURLs,
		}
	}
	return out
}
