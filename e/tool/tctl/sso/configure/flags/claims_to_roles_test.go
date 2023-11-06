package flags

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_claimsToRolesParser_Set(t *testing.T) {
	tests := []struct {
		name       string
		parser     claimsToRolesParser
		arg        string
		wantErr    bool
		wantParser claimsToRolesParser
	}{
		{
			name:   "one set of correct args",
			parser: claimsToRolesParser{mappings: &[]types.ClaimMapping{}},
			arg:    "foo,bar,baz",
			wantParser: claimsToRolesParser{mappings: &[]types.ClaimMapping{
				{
					Claim: "foo",
					Value: "bar",
					Roles: []string{"baz"},
				}}},
			wantErr: false,
		},
		{
			name: "two sets of correct args",
			parser: claimsToRolesParser{mappings: &[]types.ClaimMapping{
				{
					Claim: "foo",
					Value: "bar",
					Roles: []string{"baz"},
				}}},
			arg: "aaa,bbb,ccc,ddd",
			wantParser: claimsToRolesParser{mappings: &[]types.ClaimMapping{
				{
					Claim: "foo",
					Value: "bar",
					Roles: []string{"baz"},
				},
				{
					Claim: "aaa",
					Value: "bbb",
					Roles: []string{"ccc", "ddd"},
				}}},
			wantErr: false,
		},
		{
			name:       "one set of incorrect args",
			parser:     claimsToRolesParser{mappings: &[]types.ClaimMapping{}},
			arg:        "abracadabra",
			wantParser: claimsToRolesParser{mappings: &[]types.ClaimMapping{}},
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Set(tt.arg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantParser, tt.parser)
			}
		})
	}
}
