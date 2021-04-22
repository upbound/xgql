package model

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var (
	_ identifiable = ManagedResource{}
	_ identifiable = ProviderConfig{}
	_ identifiable = CompositeResource{}
	_ identifiable = CompositeResourceClaim{}
	_ identifiable = Provider{}
	_ identifiable = ProviderRevision{}
	_ identifiable = Configuration{}
	_ identifiable = ConfigurationRevision{}
	_ identifiable = CompositeResourceDefinition{}
	_ identifiable = Composition{}
	_ identifiable = CustomResourceDefinition{}
	_ identifiable = Secret{}
	_ identifiable = ConfigMap{}
	_ identifiable = GenericResource{}
)

var (
	_ sort.Interface = &KubernetesResourceConnection{}
	_ sort.Interface = &EventConnection{}
	_ sort.Interface = &ProviderConnection{}
	_ sort.Interface = &ProviderRevisionConnection{}
	_ sort.Interface = &CustomResourceDefinitionConnection{}
	_ sort.Interface = &ConfigurationConnection{}
	_ sort.Interface = &ConfigurationRevisionConnection{}
	_ sort.Interface = &CompositionConnection{}
	_ sort.Interface = &CompositeResourceDefinitionConnection{}
	_ sort.Interface = &CompositeResourceConnection{}
	_ sort.Interface = &CompositeResourceClaimConnection{}
)

func TestSort(t *testing.T) {
	now := time.Now()
	soon := time.Now().Add(10 * time.Second)

	cases := map[string]struct {
		conn sort.Interface
		want sort.Interface
	}{
		"KubernetesResourceConnection": {
			conn: &KubernetesResourceConnection{
				TotalCount: 2,
				Nodes: []KubernetesResource{
					GenericResource{ID: ReferenceID{Name: "b"}},
					GenericResource{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &KubernetesResourceConnection{
				TotalCount: 2,
				Nodes: []KubernetesResource{
					GenericResource{ID: ReferenceID{Name: "a"}},
					GenericResource{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"EventConnection": {
			conn: &EventConnection{
				TotalCount: 3,
				Nodes: []Event{
					{ID: ReferenceID{Name: "b"}, LastTime: &soon},
					{ID: ReferenceID{Name: "a"}, LastTime: &now},
					{ID: ReferenceID{Name: "c"}},
				},
			},
			want: &EventConnection{
				TotalCount: 3,
				Nodes: []Event{
					{ID: ReferenceID{Name: "a"}, LastTime: &now},
					{ID: ReferenceID{Name: "b"}, LastTime: &soon},
					{ID: ReferenceID{Name: "c"}},
				},
			},
		},
		"ProviderConnection": {
			conn: &ProviderConnection{
				TotalCount: 2,
				Nodes: []Provider{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &ProviderConnection{
				TotalCount: 2,
				Nodes: []Provider{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"ProviderRevisionConnection": {
			conn: &ProviderRevisionConnection{
				TotalCount: 2,
				Nodes: []ProviderRevision{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &ProviderRevisionConnection{
				TotalCount: 2,
				Nodes: []ProviderRevision{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"CustomResourceDefinitionConnection": {
			conn: &CustomResourceDefinitionConnection{
				TotalCount: 2,
				Nodes: []CustomResourceDefinition{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &CustomResourceDefinitionConnection{
				TotalCount: 2,
				Nodes: []CustomResourceDefinition{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"ConfigurationConnection": {
			conn: &ConfigurationConnection{
				TotalCount: 2,
				Nodes: []Configuration{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &ConfigurationConnection{
				TotalCount: 2,
				Nodes: []Configuration{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"ConfigurationRevisionConnection": {
			conn: &ConfigurationRevisionConnection{
				TotalCount: 2,
				Nodes: []ConfigurationRevision{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &ConfigurationRevisionConnection{
				TotalCount: 2,
				Nodes: []ConfigurationRevision{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"CompositionConnection": {
			conn: &CompositionConnection{
				TotalCount: 2,
				Nodes: []Composition{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &CompositionConnection{
				TotalCount: 2,
				Nodes: []Composition{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"CompositeResourceDefinitionConnection": {
			conn: &CompositeResourceDefinitionConnection{
				TotalCount: 2,
				Nodes: []CompositeResourceDefinition{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &CompositeResourceDefinitionConnection{
				TotalCount: 2,
				Nodes: []CompositeResourceDefinition{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"CompositeResourceConnection": {
			conn: &CompositeResourceConnection{
				TotalCount: 2,
				Nodes: []CompositeResource{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &CompositeResourceConnection{
				TotalCount: 2,
				Nodes: []CompositeResource{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
		"CompositeResourceClaimConnection": {
			conn: &CompositeResourceClaimConnection{
				TotalCount: 2,
				Nodes: []CompositeResourceClaim{
					{ID: ReferenceID{Name: "b"}},
					{ID: ReferenceID{Name: "a"}},
				},
			},
			want: &CompositeResourceClaimConnection{
				TotalCount: 2,
				Nodes: []CompositeResourceClaim{
					{ID: ReferenceID{Name: "a"}},
					{ID: ReferenceID{Name: "b"}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sort.Stable(tc.conn)
			if diff := cmp.Diff(tc.want, tc.conn); diff != "" {
				t.Errorf("sort.Stable(...): -want, +got:\n%s", diff)
			}
		})
	}

}
