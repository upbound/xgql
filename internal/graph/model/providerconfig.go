package model

import (
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/upbound/xgql/internal/unstructured"
)

// GetProviderConfig from the suppled Kubernetes ProviderConfig.
func GetProviderConfig(u *kunstructured.Unstructured) (ProviderConfig, error) {
	pc := &unstructured.ProviderConfig{Unstructured: *u}
	users := int(pc.GetUsers())

	raw, err := json.Marshal(pc)
	if err != nil {
		return ProviderConfig{}, errors.Wrap(err, "cannot marshal JSON")
	}

	out := ProviderConfig{
		APIVersion: pc.GetAPIVersion(),
		Kind:       pc.GetKind(),
		Metadata:   GetObjectMeta(pc),
		Status: &ProviderConfigStatus{
			Conditions: GetConditions(pc.GetConditions()),
			Users:      &users,
		},
		Raw: string(raw),
	}

	return out, nil
}
