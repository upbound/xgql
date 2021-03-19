package model

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// A Provider package.
type Provider struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   *ObjectMeta     `json:"metadata"`
	Spec       *ProviderSpec   `json:"spec"`
	Status     *ProviderStatus `json:"status"`
	Raw        string          `json:"raw"`
}

// IsKubernetesResource indicates that Provider satisfies the KubernetesResource
// GraphQL (and corresponding Go) interface.
func (Provider) IsKubernetesResource() {}

// A ProviderSpec specifies the desired state of a Provider.
type ProviderSpec struct {
	Package                     string                    `json:"package"`
	RevisionActivationPolicy    *RevisionActivationPolicy `json:"revisionActivationPolicy"`
	RevisionHistoryLimit        *int                      `json:"revisionHistoryLimit"`
	PackagePullPolicy           *PackagePullPolicy        `json:"packagePullPolicy"`
	IgnoreCrossplaneConstraints *bool                     `json:"ignoreCrossplaneConstraints"`
	SkipDependencyResolution    *bool                     `json:"skipDependencyResolution"`

	PackagePullSecrets []corev1.LocalObjectReference
}

// A ProviderStatus reflects the observed state of a Provider.
type ProviderStatus struct {
	Conditions        []Condition `json:"conditions"`
	CurrentRevision   *string     `json:"currentRevision"`
	CurrentIdentifier *string     `json:"currentIdentifier"`
}

// IsConditionedStatus indicates that ProviderStatus satisfies the
// ConditionedStatus GraphQL (and corresponding Go) interface.
func (ProviderStatus) IsConditionedStatus() {}

// A ProviderRevision of a Provider package.
type ProviderRevision struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Metadata   *ObjectMeta             `json:"metadata"`
	Spec       *ProviderRevisionSpec   `json:"spec"`
	Status     *ProviderRevisionStatus `json:"status"`
	Raw        string                  `json:"raw"`
}

// IsKubernetesResource reflects that ProviderRevision satisfies the
// KubernetesResource GraphQL (and corresponding Go) interface.
func (ProviderRevision) IsKubernetesResource() {}

// A ProviderRevisionSpec specifies the desired state of a ProviderRevision.
type ProviderRevisionSpec struct {
	DesiredState                PackageRevisionDesiredState `json:"desiredState"`
	Package                     string                      `json:"package"`
	PackagePullPolicy           *PackagePullPolicy          `json:"packagePullPolicy"`
	Revision                    int                         `json:"revision"`
	IgnoreCrossplaneConstraints *bool                       `json:"ignoreCrossplaneConstraints"`
	SkipDependencyResolution    *bool                       `json:"skipDependencyResolution"`

	PackagePullSecrets []corev1.LocalObjectReference
}

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
