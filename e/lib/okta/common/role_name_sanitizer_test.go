package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOktaResourceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "Okta Group Name (Production !!!)",
			want:  "okta_group_name_production",
		},
		{
			input: "  group  name   ",
			want:  "group_name",
		},
		{
			input: "___  group  name   ___",
			want:  "group_name",
		},
		{
			input: "a _ b",
			want:  "a_b",
		},
		{
			input: "../group ąćłę name  +_!@#$%^|} prod ",
			want:  "group_name_@_prod",
		},
		{
			input: " @prod_group_name 1",
			want:  "@prod_group_name_1",
		},
		{
			input: "new line \n group",
			want:  "new_line_group",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeOktaResourceName(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestOktaResourceToTeleportName(t *testing.T) {
	tests := []struct {
		name             string
		inputDisplayName string
		inputID          string
		want             string
	}{
		{
			name:             "empty display name",
			inputDisplayName: "",
			inputID:          "12323",
			want:             "access-okta-acl-role-12323",
		},
		{
			name:             "display name and id ",
			inputDisplayName: " App Group 1 (Production Dev)",
			inputID:          "12323",
			want:             "app_group_1_production_dev-access-okta-acl-role-12323",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CreateOktaAccessRoleFriendlyName(tc.inputDisplayName, tc.inputID)
			require.Equal(t, tc.want, got)
		})
	}
}
