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

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/google/go-cmp/cmp"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// GetRevisionActivationPolicy from the supplied Crossplane policy.
func GetRevisionActivationPolicy(in *pkgv1.RevisionActivationPolicy) *RevisionActivationPolicy {
	if in == nil {
		return nil
	}
	switch *in {
	case pkgv1.ManualActivation:
		out := RevisionActivationPolicyManual
		return &out
	case pkgv1.AutomaticActivation:
		out := RevisionActivationPolicyAutomatic
		return &out
	}
	return nil
}

// GetPackagePullPolicy from the supplied Kubernetes policy.
func GetPackagePullPolicy(in *corev1.PullPolicy) *PackagePullPolicy {
	if in == nil {
		return nil
	}
	switch *in {
	case corev1.PullAlways:
		out := PackagePullPolicyAlways
		return &out
	case corev1.PullNever:
		out := PackagePullPolicyNever
		return &out
	case corev1.PullIfNotPresent:
		out := PackagePullPolicyIfNotPresent
		return &out
	}
	return nil
}

// GetPolicyRules from the supplied Kubernetes policy rules.
func GetPolicyRules(in []rbacv1.PolicyRule) []PolicyRule {
	if in == nil {
		return nil
	}

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

// GetProviderStatus from the supplied Kubernetes status.
func GetProviderStatus(in pkgv1.ProviderStatus) *ProviderStatus {
	out := &ProviderStatus{Conditions: GetConditions(in.Conditions)}
	if in.CurrentRevision != "" {
		out.CurrentRevision = &in.CurrentRevision
	}
	if in.CurrentIdentifier != "" {
		out.CurrentIdentifier = &in.CurrentIdentifier
	}
	if cmp.Equal(out, &ProviderStatus{}) {
		return nil
	}
	return out
}

// GetProvider from the supplied Kubernetes provider.
func GetProvider(p *pkgv1.Provider) Provider {
	return Provider{
		ID: ReferenceID{
			APIVersion: p.APIVersion,
			Kind:       p.Kind,
			Name:       p.GetName(),
		},

		APIVersion: p.APIVersion,
		Kind:       p.Kind,
		Metadata:   GetObjectMeta(p),
		Spec: ProviderSpec{
			Package:                     p.Spec.Package,
			RevisionActivationPolicy:    GetRevisionActivationPolicy(p.Spec.RevisionActivationPolicy),
			RevisionHistoryLimit:        getIntPtr(p.Spec.RevisionHistoryLimit),
			PackagePullPolicy:           GetPackagePullPolicy(p.Spec.PackagePullPolicy),
			IgnoreCrossplaneConstraints: p.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    p.Spec.SkipDependencyResolution,
		},
		Status: GetProviderStatus(p.Status),
		PavedAccess: PavedAccess{
			Paved: paveObject(p),
		},
	}
}

// GetPackageRevisionDesiredState from the supplies Crossplane state.
func GetPackageRevisionDesiredState(in pkgv1.PackageRevisionDesiredState) PackageRevisionDesiredState {
	switch in {
	case pkgv1.PackageRevisionActive:
		return PackageRevisionDesiredStateActive
	case pkgv1.PackageRevisionInactive:
		return PackageRevisionDesiredStateInactive
	}
	return ""
}

// GetProviderRevisionStatus from the supplied Crossplane provider revision.
func GetProviderRevisionStatus(in pkgv1.PackageRevisionStatus) *ProviderRevisionStatus {
	out := &ProviderRevisionStatus{
		Conditions:            GetConditions(in.Conditions),
		ObjectRefs:            in.ObjectRefs,
		FoundDependencies:     getIntPtr(&in.FoundDependencies),
		InstalledDependencies: getIntPtr(&in.InstalledDependencies),
		InvalidDependencies:   getIntPtr(&in.InvalidDependencies),
		PermissionRequests:    GetPolicyRules(in.PermissionRequests),
	}
	if cmp.Equal(out, &ProviderRevisionStatus{}) {
		return nil
	}
	return out
}

// GetProviderRevision from the supplied Crossplane provider revision.
func GetProviderRevision(pr *pkgv1.ProviderRevision) ProviderRevision {
	return ProviderRevision{
		ID: ReferenceID{
			APIVersion: pr.APIVersion,
			Kind:       pr.Kind,
			Name:       pr.GetName(),
		},

		APIVersion: pr.APIVersion,
		Kind:       pr.Kind,
		Metadata:   GetObjectMeta(pr),
		Spec: ProviderRevisionSpec{
			DesiredState:                GetPackageRevisionDesiredState(pr.Spec.DesiredState),
			Package:                     pr.Spec.Package,
			PackagePullPolicy:           GetPackagePullPolicy(pr.Spec.PackagePullPolicy),
			Revision:                    int(pr.Spec.Revision),
			IgnoreCrossplaneConstraints: pr.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    pr.Spec.SkipDependencyResolution,
		},
		Status: GetProviderRevisionStatus(pr.Status),
		PavedAccess: PavedAccess{
			Paved: paveObject(pr),
		},
	}
}

// GetConfigurationStatus from the supplied Kubernetes status.
func GetConfigurationStatus(in pkgv1.ConfigurationStatus) *ConfigurationStatus {
	out := &ConfigurationStatus{Conditions: GetConditions(in.Conditions)}
	if in.CurrentRevision != "" {
		out.CurrentRevision = &in.CurrentRevision
	}
	if in.CurrentIdentifier != "" {
		out.CurrentIdentifier = &in.CurrentIdentifier
	}
	if cmp.Equal(out, &ConfigurationStatus{}) {
		return nil
	}
	return out
}

// GetConfiguration from the supplied Kubernetes configuration.
func GetConfiguration(c *pkgv1.Configuration) Configuration {
	return Configuration{
		ID: ReferenceID{
			APIVersion: c.APIVersion,
			Kind:       c.Kind,
			Name:       c.GetName(),
		},

		APIVersion: c.APIVersion,
		Kind:       c.Kind,
		Metadata:   GetObjectMeta(c),
		Spec: ConfigurationSpec{
			Package:                     c.Spec.Package,
			RevisionActivationPolicy:    GetRevisionActivationPolicy(c.Spec.RevisionActivationPolicy),
			RevisionHistoryLimit:        getIntPtr(c.Spec.RevisionHistoryLimit),
			PackagePullPolicy:           GetPackagePullPolicy(c.Spec.PackagePullPolicy),
			IgnoreCrossplaneConstraints: c.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    c.Spec.SkipDependencyResolution,
		},
		Status: GetConfigurationStatus(c.Status),
		PavedAccess: PavedAccess{
			Paved: paveObject(c),
		},
	}
}

// GetConfigurationRevisionStatus from the supplied Crossplane provider revision.
func GetConfigurationRevisionStatus(in pkgv1.PackageRevisionStatus) *ConfigurationRevisionStatus {
	out := &ConfigurationRevisionStatus{
		Conditions:            GetConditions(in.Conditions),
		ObjectRefs:            in.ObjectRefs,
		FoundDependencies:     getIntPtr(&in.FoundDependencies),
		InstalledDependencies: getIntPtr(&in.InstalledDependencies),
		InvalidDependencies:   getIntPtr(&in.InvalidDependencies),
		PermissionRequests:    GetPolicyRules(in.PermissionRequests),
	}
	if cmp.Equal(out, &ConfigurationRevisionStatus{}) {
		return nil
	}
	return out
}

// GetConfigurationRevision from the supplied Kubernetes provider revision.
func GetConfigurationRevision(cr *pkgv1.ConfigurationRevision) ConfigurationRevision {
	return ConfigurationRevision{
		ID: ReferenceID{
			APIVersion: cr.APIVersion,
			Kind:       cr.Kind,
			Name:       cr.GetName(),
		},

		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Metadata:   GetObjectMeta(cr),
		Spec: ConfigurationRevisionSpec{
			DesiredState:                GetPackageRevisionDesiredState(cr.Spec.DesiredState),
			Package:                     cr.Spec.Package,
			PackagePullPolicy:           GetPackagePullPolicy(cr.Spec.PackagePullPolicy),
			Revision:                    int(cr.Spec.Revision),
			IgnoreCrossplaneConstraints: cr.Spec.IgnoreCrossplaneConstraints,
			SkipDependencyResolution:    cr.Spec.SkipDependencyResolution,
		},
		Status: GetConfigurationRevisionStatus(cr.Status),
		PavedAccess: PavedAccess{
			Paved: paveObject(cr),
		},
	}
}
