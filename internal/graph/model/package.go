package model

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

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

// GetProvider from the supplied Kubernetes provider.
func GetProvider(p *pkgv1.Provider) (Provider, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return Provider{}, errors.Wrap(err, "could not marshal JSON")
	}

	out := Provider{
		ID: ReferenceID{
			APIVersion: p.APIVersion,
			Kind:       p.Kind,
			Name:       p.GetName(),
		},

		APIVersion: p.APIVersion,
		Kind:       p.Kind,
		Metadata:   GetObjectMeta(p),
		Spec: &ProviderSpec{
			Package:                     p.Spec.Package,
			RevisionActivationPolicy:    GetRevisionActivationPolicy(p.Spec.RevisionActivationPolicy),
			RevisionHistoryLimit:        getIntPtr(p.Spec.RevisionHistoryLimit),
			PackagePullPolicy:           GetPackagePullPolicy(p.Spec.PackagePullPolicy),
			IgnoreCrossplaneConstraints: p.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    p.Spec.SkipDependencyResolution,
		},
		Status: &ProviderStatus{
			Conditions:        GetConditions(p.Status.Conditions),
			CurrentRevision:   pointer.StringPtr(p.Status.CurrentRevision),
			CurrentIdentifier: &p.Status.CurrentIdentifier,
		},
		Raw: string(raw),
	}

	return out, nil
}

// GetProviderRevision from the supplied Kubernetes provider revision.
func GetProviderRevision(pr *pkgv1.ProviderRevision) (ProviderRevision, error) {
	raw, err := json.Marshal(pr)
	if err != nil {
		return ProviderRevision{}, errors.Wrap(err, "could not marshal JSON")
	}

	out := ProviderRevision{
		ID: ReferenceID{
			APIVersion: pr.APIVersion,
			Kind:       pr.Kind,
			Name:       pr.GetName(),
		},

		APIVersion: pr.APIVersion,
		Kind:       pr.Kind,
		Metadata:   GetObjectMeta(pr),
		Spec: &ProviderRevisionSpec{
			DesiredState:                PackageRevisionDesiredState(pr.Spec.DesiredState),
			Package:                     pr.Spec.Package,
			PackagePullPolicy:           GetPackagePullPolicy(pr.Spec.PackagePullPolicy),
			Revision:                    int(pr.Spec.Revision),
			IgnoreCrossplaneConstraints: pr.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    pr.Spec.SkipDependencyResolution,
		},
		Status: &ProviderRevisionStatus{
			Conditions:            GetConditions(pr.Status.Conditions),
			FoundDependencies:     getIntPtr(&pr.Status.FoundDependencies),
			InstalledDependencies: getIntPtr(&pr.Status.InstalledDependencies),
			InvalidDependencies:   getIntPtr(&pr.Status.InvalidDependencies),
			PermissionRequests:    GetPolicyRules(pr.Status.PermissionRequests),
			ObjectRefs:            pr.Status.ObjectRefs,
		},
		Raw: string(raw),
	}

	return out, nil
}

// GetConfiguration from the supplied Kubernetes configuration.
func GetConfiguration(c *pkgv1.Configuration) (Configuration, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return Configuration{}, errors.Wrap(err, "could not marshal JSON")
	}

	out := Configuration{
		ID: ReferenceID{
			APIVersion: c.APIVersion,
			Kind:       c.Kind,
			Name:       c.GetName(),
		},

		APIVersion: c.APIVersion,
		Kind:       c.Kind,
		Metadata:   GetObjectMeta(c),
		Spec: &ConfigurationSpec{
			Package:                     c.Spec.Package,
			RevisionActivationPolicy:    GetRevisionActivationPolicy(c.Spec.RevisionActivationPolicy),
			RevisionHistoryLimit:        getIntPtr(c.Spec.RevisionHistoryLimit),
			PackagePullPolicy:           GetPackagePullPolicy(c.Spec.PackagePullPolicy),
			IgnoreCrossplaneConstraints: c.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    c.Spec.SkipDependencyResolution,
		},
		Status: &ConfigurationStatus{
			Conditions:        GetConditions(c.Status.Conditions),
			CurrentRevision:   pointer.StringPtr(c.Status.CurrentRevision),
			CurrentIdentifier: &c.Status.CurrentIdentifier,
		},
		Raw: string(raw),
	}

	return out, nil
}

// GetConfigurationRevision from the supplied Kubernetes provider revision.
func GetConfigurationRevision(cr *pkgv1.ConfigurationRevision) (ConfigurationRevision, error) {
	raw, err := json.Marshal(cr)
	if err != nil {
		return ConfigurationRevision{}, errors.Wrap(err, "could not marshal JSON")
	}

	out := ConfigurationRevision{
		ID: ReferenceID{
			APIVersion: cr.APIVersion,
			Kind:       cr.Kind,
			Name:       cr.GetName(),
		},

		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Metadata:   GetObjectMeta(cr),
		Spec: &ConfigurationRevisionSpec{
			DesiredState:                PackageRevisionDesiredState(cr.Spec.DesiredState),
			Package:                     cr.Spec.Package,
			PackagePullPolicy:           GetPackagePullPolicy(cr.Spec.PackagePullPolicy),
			Revision:                    int(cr.Spec.Revision),
			IgnoreCrossplaneConstraints: cr.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    cr.Spec.SkipDependencyResolution,
		},
		Status: &ConfigurationRevisionStatus{
			Conditions:            GetConditions(cr.Status.Conditions),
			FoundDependencies:     getIntPtr(&cr.Status.FoundDependencies),
			InstalledDependencies: getIntPtr(&cr.Status.InstalledDependencies),
			InvalidDependencies:   getIntPtr(&cr.Status.InvalidDependencies),
			PermissionRequests:    GetPolicyRules(cr.Status.PermissionRequests),
			ObjectRefs:            cr.Status.ObjectRefs,
		},
		Raw: string(raw),
	}

	return out, nil
}
