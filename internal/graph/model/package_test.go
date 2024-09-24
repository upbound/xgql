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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestGetProvider(t *testing.T) {
	rap := pkgv1.ManualActivation
	lim := int64(42)
	ppp := corev1.PullAlways

	mrap := RevisionActivationPolicyManual
	mlim := 42
	mppp := PackagePullPolicyAlways

	cases := map[string]struct {
		reason string
		cfg    *pkgv1.Provider
		want   Provider
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			cfg: &pkgv1.Provider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: pkgv1.ProviderGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ProviderKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: pkgv1.ProviderSpec{
					PackageSpec: pkgv1.PackageSpec{
						Package:                     "coolthing:v1",
						RevisionActivationPolicy:    &rap,
						RevisionHistoryLimit:        &lim,
						PackagePullPolicy:           &ppp,
						IgnoreCrossplaneConstraints: ptr.To(true),
						SkipDependencyResolution:    ptr.To(true),
					},
				},
				Status: pkgv1.ProviderStatus{
					ConditionedStatus: xpv1.ConditionedStatus{
						Conditions: []xpv1.Condition{{}},
					},
					PackageStatus: pkgv1.PackageStatus{
						CurrentRevision:   "8",
						CurrentIdentifier: "coolthing:v1",
					},
				},
			},
			want: Provider{
				ID: ReferenceID{
					APIVersion: pkgv1.ProviderGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ProviderKind,
					Name:       "cool",
				},
				APIVersion: pkgv1.ProviderGroupVersionKind.GroupVersion().String(),
				Kind:       pkgv1.ProviderKind,
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: ProviderSpec{
					Package:                     "coolthing:v1",
					RevisionActivationPolicy:    &mrap,
					RevisionHistoryLimit:        &mlim,
					PackagePullPolicy:           &mppp,
					IgnoreCrossplaneConstraints: ptr.To(true),
					SkipDependencyResolution:    ptr.To(true),
				},
				Status: &ProviderStatus{
					Conditions:        []Condition{{}},
					CurrentRevision:   ptr.To("8"),
					CurrentIdentifier: ptr.To("coolthing:v1"),
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			cfg:    &pkgv1.Provider{},
			want: Provider{
				Metadata: ObjectMeta{},
				Spec:     ProviderSpec{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetProvider(tc.cfg)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Provider{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetProvider(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetProviderRevision(t *testing.T) {
	ppp := corev1.PullAlways
	mppp := PackagePullPolicyAlways
	found := 42
	installed := 32
	invalid := 10

	cases := map[string]struct {
		reason string
		cfg    *pkgv1.ProviderRevision
		want   ProviderRevision
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			cfg: &pkgv1.ProviderRevision{
				TypeMeta: metav1.TypeMeta{
					APIVersion: pkgv1.ProviderRevisionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ProviderRevisionKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: pkgv1.ProviderRevisionSpec{
					PackageRevisionSpec: pkgv1.PackageRevisionSpec{
						DesiredState:                pkgv1.PackageRevisionActive,
						Package:                     "coolthing:v1",
						PackagePullPolicy:           &ppp,
						Revision:                    42,
						IgnoreCrossplaneConstraints: ptr.To(true),
						SkipDependencyResolution:    ptr.To(true),
					},
				},
				Status: pkgv1.PackageRevisionStatus{
					ConditionedStatus: xpv1.ConditionedStatus{
						Conditions: []xpv1.Condition{{}},
					},
					ObjectRefs:            []xpv1.TypedReference{{Name: "coolcrd"}},
					FoundDependencies:     int64(found),
					InstalledDependencies: int64(installed),
					InvalidDependencies:   int64(invalid),
					PermissionRequests: []rbacv1.PolicyRule{{
						Verbs:           []string{"verb"},
						APIGroups:       []string{"group"},
						Resources:       []string{"resources"},
						ResourceNames:   []string{"resourceNames"},
						NonResourceURLs: []string{"nonResourceURLs"},
					}},
				},
			},
			want: ProviderRevision{
				ID: ReferenceID{
					APIVersion: pkgv1.ProviderRevisionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ProviderRevisionKind,
					Name:       "cool",
				},
				APIVersion: pkgv1.ProviderRevisionGroupVersionKind.GroupVersion().String(),
				Kind:       pkgv1.ProviderRevisionKind,
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: ProviderRevisionSpec{
					DesiredState:                PackageRevisionDesiredStateActive,
					Package:                     "coolthing:v1",
					PackagePullPolicy:           &mppp,
					Revision:                    42,
					IgnoreCrossplaneConstraints: ptr.To(true),
					SkipDependencyResolution:    ptr.To(true),
				},
				Status: &ProviderRevisionStatus{
					Conditions:            []Condition{{}},
					ObjectRefs:            []xpv1.TypedReference{{Name: "coolcrd"}},
					FoundDependencies:     &found,
					InstalledDependencies: &installed,
					InvalidDependencies:   &invalid,
					PermissionRequests: []PolicyRule{{
						Verbs:           []string{"verb"},
						APIGroups:       []string{"group"},
						Resources:       []string{"resources"},
						ResourceNames:   []string{"resourceNames"},
						NonResourceURLs: []string{"nonResourceURLs"},
					}},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			cfg:    &pkgv1.ProviderRevision{},
			want: ProviderRevision{
				Metadata: ObjectMeta{},
				Spec:     ProviderRevisionSpec{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetProviderRevision(tc.cfg)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(ProviderRevision{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetProviderRevision(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetConfiguration(t *testing.T) {
	rap := pkgv1.ManualActivation
	lim := int64(42)
	ppp := corev1.PullAlways

	mrap := RevisionActivationPolicyManual
	mlim := 42
	mppp := PackagePullPolicyAlways

	cases := map[string]struct {
		reason string
		cfg    *pkgv1.Configuration
		want   Configuration
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			cfg: &pkgv1.Configuration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: pkgv1.ConfigurationGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ConfigurationKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: pkgv1.ConfigurationSpec{
					PackageSpec: pkgv1.PackageSpec{
						Package:                     "coolthing:v1",
						RevisionActivationPolicy:    &rap,
						RevisionHistoryLimit:        &lim,
						PackagePullPolicy:           &ppp,
						IgnoreCrossplaneConstraints: ptr.To(true),
						SkipDependencyResolution:    ptr.To(true),
					},
				},
				Status: pkgv1.ConfigurationStatus{
					ConditionedStatus: xpv1.ConditionedStatus{
						Conditions: []xpv1.Condition{{}},
					},
					PackageStatus: pkgv1.PackageStatus{
						CurrentRevision:   "8",
						CurrentIdentifier: "coolthing:v1",
					},
				},
			},
			want: Configuration{
				ID: ReferenceID{
					APIVersion: pkgv1.ConfigurationGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ConfigurationKind,
					Name:       "cool",
				},
				APIVersion: pkgv1.ConfigurationGroupVersionKind.GroupVersion().String(),
				Kind:       pkgv1.ConfigurationKind,
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: ConfigurationSpec{
					Package:                     "coolthing:v1",
					RevisionActivationPolicy:    &mrap,
					RevisionHistoryLimit:        &mlim,
					PackagePullPolicy:           &mppp,
					IgnoreCrossplaneConstraints: ptr.To(true),
					SkipDependencyResolution:    ptr.To(true),
				},
				Status: &ConfigurationStatus{
					Conditions:        []Condition{{}},
					CurrentRevision:   ptr.To("8"),
					CurrentIdentifier: ptr.To("coolthing:v1"),
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			cfg:    &pkgv1.Configuration{},
			want: Configuration{
				Metadata: ObjectMeta{},
				Spec:     ConfigurationSpec{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetConfiguration(tc.cfg)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(Configuration{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetConfiguration(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}

func TestGetConfigurationRevision(t *testing.T) {
	ppp := corev1.PullAlways
	mppp := PackagePullPolicyAlways
	found := 42
	installed := 32
	invalid := 10

	cases := map[string]struct {
		reason string
		cfg    *pkgv1.ConfigurationRevision
		want   ConfigurationRevision
	}{
		"Full": {
			reason: "All supported fields should be converted to our model",
			cfg: &pkgv1.ConfigurationRevision{
				TypeMeta: metav1.TypeMeta{
					APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ConfigurationRevisionKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool",
				},
				Spec: pkgv1.PackageRevisionSpec{
					DesiredState:                pkgv1.PackageRevisionActive,
					Package:                     "coolthing:v1",
					PackagePullPolicy:           &ppp,
					Revision:                    42,
					IgnoreCrossplaneConstraints: ptr.To(true),
					SkipDependencyResolution:    ptr.To(true),
				},
				Status: pkgv1.PackageRevisionStatus{
					ConditionedStatus: xpv1.ConditionedStatus{
						Conditions: []xpv1.Condition{{}},
					},
					ObjectRefs:            []xpv1.TypedReference{{Name: "coolcrd"}},
					FoundDependencies:     int64(found),
					InstalledDependencies: int64(installed),
					InvalidDependencies:   int64(invalid),
					PermissionRequests: []rbacv1.PolicyRule{{
						Verbs:           []string{"verb"},
						APIGroups:       []string{"group"},
						Resources:       []string{"resources"},
						ResourceNames:   []string{"resourceNames"},
						NonResourceURLs: []string{"nonResourceURLs"},
					}},
				},
			},
			want: ConfigurationRevision{
				ID: ReferenceID{
					APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.ConfigurationRevisionKind,
					Name:       "cool",
				},
				APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
				Kind:       pkgv1.ConfigurationRevisionKind,
				Metadata: ObjectMeta{
					Name: "cool",
				},
				Spec: ConfigurationRevisionSpec{
					DesiredState:                PackageRevisionDesiredStateActive,
					Package:                     "coolthing:v1",
					PackagePullPolicy:           &mppp,
					Revision:                    42,
					IgnoreCrossplaneConstraints: ptr.To(true),
					SkipDependencyResolution:    ptr.To(true),
				},
				Status: &ConfigurationRevisionStatus{
					Conditions:            []Condition{{}},
					ObjectRefs:            []xpv1.TypedReference{{Name: "coolcrd"}},
					FoundDependencies:     &found,
					InstalledDependencies: &installed,
					InvalidDependencies:   &invalid,
					PermissionRequests: []PolicyRule{{
						Verbs:           []string{"verb"},
						APIGroups:       []string{"group"},
						Resources:       []string{"resources"},
						ResourceNames:   []string{"resourceNames"},
						NonResourceURLs: []string{"nonResourceURLs"},
					}},
				},
			},
		},
		"Empty": {
			reason: "Absent optional fields should be absent in our model",
			cfg:    &pkgv1.ConfigurationRevision{},
			want: ConfigurationRevision{
				Metadata: ObjectMeta{},
				Spec:     ConfigurationRevisionSpec{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetConfigurationRevision(tc.cfg)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(ConfigurationRevision{}, "PavedAccess"), cmp.AllowUnexported(ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nGetConfigurationRevision(...): -want, +got\n:%s", tc.reason, diff)
			}
		})
	}
}
