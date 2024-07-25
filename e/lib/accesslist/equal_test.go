package accesslist

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestAccessListEqual(t *testing.T) {
	tests := []struct {
		name      string
		first     *accesslist.AccessList
		second    *accesslist.AccessList
		wantEqual bool
	}{
		{
			name:      "both nil",
			first:     nil,
			second:    nil,
			wantEqual: true,
		},
		{
			name: "nil and empty slice",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles:  []string{},
						Traits: map[string][]string{},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles:  nil,
						Traits: nil,
					},
				},
			},
			wantEqual: true,
		},
		{
			name: "nil and no empty slice",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles: []string{"role1"},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Roles: nil,
					},
				},
			},
			wantEqual: false,
		},
		{
			name: "nil and no empty slice",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{
						Traits: map[string][]string{"trait1": {"value1"}},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					OwnershipRequires: accesslist.Requires{},
				},
			},
			wantEqual: false,
		},
		{
			name: "ephemeral fields",
			first: &accesslist.AccessList{
				Spec: accesslist.Spec{
					Owners: []accesslist.Owner{
						{
							IneligibleStatus: "ineligible",
						},
					},
				},
			},
			second: &accesslist.AccessList{
				Spec: accesslist.Spec{
					Owners: []accesslist.Owner{
						{
							IneligibleStatus: "not-ineligible",
						},
					},
				},
			},
			wantEqual: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := accessListEqual(tc.first, tc.second)
			require.Equal(t, tc.wantEqual, got)
		})
	}
}
