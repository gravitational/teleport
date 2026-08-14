package web

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestGenerateAccessListTerraformConfig(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	endpoint := webPack.clt.Endpoint("enterprise", "generate", "terraform", "accesslist")

	accessRole := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "accessRole1",
		},
	}

	accessRole2 := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "accessRole2",
		},
	}

	accessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "test-access-list"},
		accesslist.Spec{
			Title:  "Test Access List",
			Owners: []accesslist.Owner{{Name: "llama", Description: "some description"}},
			Audit: accesslist.Audit{
				Recurrence: accesslist.Recurrence{
					Frequency:  accesslist.ThreeMonths,
					DayOfMonth: accesslist.FirstDayOfMonth,
				},
			},
		},
	)
	require.NoError(t, err)

	members := []accesslist.AccessListMemberSpec{
		{
			Name: "llama",
		},
		{
			Name:   "alpaca",
			Reason: "some reason",
		},
	}

	replaceDynamicTerraformProviderValues := func(output string) string {
		output = strings.ReplaceAll(output, s.webServer.Listener.Addr().String(), "proxy.example.com:3080")
		output = strings.ReplaceAll(output, fmt.Sprintf("~> %d.0", teleport.SemVer().Major), "~> 0.0")
		return output
	}

	t.Run("long-term-preset", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:   "long-term",
			AccessListID: accessList.GetName(),
			AccessList:   &ui.AccessList{AccessList: accessList, Members: members},
			AccessRoles: []ui.AccessRole{
				{Role: accessRole, BlockComment: "Some kind of block comment"},
				{Role: accessRole2, BlockComment: "Some kind of block comment for role 2"},
			},
		}
		resp, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.NoError(t, err)

		var result ui.GenerateAccessListTerraformConfigResponse
		require.NoError(t, json.Unmarshal(resp.Bytes(), &result))

		got := []byte(replaceDynamicTerraformProviderValues(result.Terraform))
		if golden.ShouldSet() {
			golden.Set(t, got)
		}
		require.Equal(t, string(golden.Get(t)), string(got))
	})

	t.Run("short-term-preset", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:   "short-term",
			AccessListID: accessList.GetName(),
			AccessList:   &ui.AccessList{AccessList: accessList, Members: members},
			AccessRoles:  []ui.AccessRole{{Role: accessRole, BlockComment: "Some kind of block comment"}},
		}
		resp, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.NoError(t, err)

		var result ui.GenerateAccessListTerraformConfigResponse
		require.NoError(t, json.Unmarshal(resp.Bytes(), &result))

		got := []byte(replaceDynamicTerraformProviderValues(result.Terraform))
		if golden.ShouldSet() {
			golden.Set(t, got)
		}
		require.Equal(t, string(golden.Get(t)), string(got))
	})

	t.Run("access-roles-nil-access-list", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:   "long-term",
			AccessListID: accessList.GetName(),
			AccessRoles: []ui.AccessRole{
				{Role: accessRole, BlockComment: "First access role"},
				{Role: accessRole2, BlockComment: "Second access role"},
			},
		}
		resp, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.NoError(t, err)

		var result ui.GenerateAccessListTerraformConfigResponse
		require.NoError(t, json.Unmarshal(resp.Bytes(), &result))

		got := []byte(replaceDynamicTerraformProviderValues(result.Terraform))
		if golden.ShouldSet() {
			golden.Set(t, got)
		}
		require.Equal(t, string(golden.Get(t)), string(got))
	})

	t.Run("no-access-roles", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:   "long-term",
			AccessListID: accessList.GetName(),
			AccessList:   &ui.AccessList{AccessList: accessList, Members: members},
		}
		resp, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.NoError(t, err)

		var result ui.GenerateAccessListTerraformConfigResponse
		require.NoError(t, json.Unmarshal(resp.Bytes(), &result))

		got := []byte(replaceDynamicTerraformProviderValues(result.Terraform))
		if golden.ShouldSet() {
			golden.Set(t, got)
		}
		require.Equal(t, string(golden.Get(t)), string(got))
	})

	t.Run("missing accessListId", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:  "short-term",
			AccessList:  &ui.AccessList{AccessList: accessList},
			AccessRoles: []ui.AccessRole{{Role: accessRole}},
		}
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.Error(t, err)
	})

	t.Run("invalid preset type", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:   "invalid",
			AccessListID: accessList.GetName(),
		}
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.Error(t, err)
	})

	t.Run("invalid access list name match", func(t *testing.T) {
		req := ui.GenerateAccessListTerraformConfigRequest{
			PresetType:   "short-term",
			AccessListID: "some-other-name",
			AccessList:   &ui.AccessList{AccessList: accessList},
		}
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, req)
		require.Error(t, err)
	})
}
