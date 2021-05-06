package unstructured

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// ProbablyProviderConfig returns true if the supplied *Unstructured is probably
// a provider config. It considers any cluster scoped resource of kind:
// ProviderConfig to probably be a provider config.
func ProbablyProviderConfig(u *unstructured.Unstructured) bool {
	return u.GetNamespace() == "" && u.GetKind() == "ProviderConfig"
}

// A ProviderConfig resource.
type ProviderConfig struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *Unstructured.
func (u *ProviderConfig) GetUnstructured() *unstructured.Unstructured {
	return &u.Unstructured
}

// GetUsers of this provider config.
func (u *ProviderConfig) GetUsers() int64 {
	out := int64(0)
	_ = fieldpath.Pave(u.Object).GetValueInto("status.users", &out)
	return out
}

// SetUsers of this provider config.
func (u *ProviderConfig) SetUsers(i int64) {
	_ = fieldpath.Pave(u.Object).SetValue("status.users", i)
}

// GetCondition of this provider config.
func (u *ProviderConfig) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(u.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this provider config.
func (u *ProviderConfig) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(u.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(u.Object).SetValue("status.conditions", conditioned.Conditions)
}

// NOTE(negz): The below method isn't part of the resource.ProviderConfig
// interface; it exists to allow us to extract conditions to convert to our
// GraphQL model.

// GetConditions of this provider config.
func (u *ProviderConfig) GetConditions() []xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(u.Object).GetValueInto("status", &conditioned); err != nil {
		return nil
	}
	return conditioned.Conditions
}
