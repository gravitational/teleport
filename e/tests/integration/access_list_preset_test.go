package integration

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/e/tests/common"
	libslice "github.com/gravitational/teleport/lib/utils/slices"
)

func TestAccessListWithPreset(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../fixtures/license-eub.pem"),
		common.WithRole(t, "nop-role"),
		common.WithUser(t, "admin", "editor"),
		common.WithUser(t, "bob", "nop-role"),
		common.WithUser(t, "alice", "nop-role"),
		common.WithApp("dev-app", "http://localhost:443", map[string]string{"env": "dev"}),
		common.WithApp("prod-app", "http://localhost:443", map[string]string{"env": "prod"}),
	)

	t.Run("test short turn", func(t *testing.T) {
		testShortTerm(t, sut)
	})

	t.Run("test long term", func(t *testing.T) {
		testShortLongTermDelete(t, sut)
	})
}

func testShortLongTermDelete(t *testing.T, sut *common.SUT) {
	ctx := t.Context()
	adminClient := sut.CreateWebClientForUser(t, "admin")
	endpoint := adminClient.Endpoint("enterprise", "accesslistpreset")

	bobClient := sut.CreateWebClientForUser(t, "bob")
	resources := common.MustListUnifedResources(t, bobClient, common.WithSearchAsRole())
	require.Empty(t, resources.Items, "bob should have no searchable resources initially")

	req := buildAccessListWithPresetRequest()
	req.PresetType = "long-term"
	resp, err := common.Roundtrip[ui.AccessListWithPresetResponse](ctx, adminClient, http.MethodPost, endpoint, req)
	require.NoError(t, err)

	mustDeleteAccessListAndRoles(t, sut, resp)
}

func buildAccessListWithPresetRequest() ui.AccessListWithPresetRequest {
	acl := accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: "dev-jit-access"},
		},
		Spec: accesslist.Spec{
			Title:  "Dev JIT Access List",
			Owners: []accesslist.Owner{{Name: "alice"}},
		},
	}
	accessRoles := []*types.RoleV6{{
		Metadata: types.Metadata{Name: "dev"},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				AppLabels: types.Labels{"env": []string{"dev"}},
			},
		},
	}}
	return ui.AccessListWithPresetRequest{
		AccessList: &ui.AccessList{
			AccessList: &acl,
			Members: []accesslist.AccessListMemberSpec{
				{Name: "bob"},
			},
			MembersCount:           nil,
			MemberListCount:        nil,
			InheritedMemberGrants:  accesslist.Grants{},
			CurrentUserAssignments: nil,
			UserAssignments:        nil,
		},
		PresetType:  "short-term",
		AccessRoles: accessRoles,
	}
}

func testShortTerm(t *testing.T, sut *common.SUT) {
	ctx := t.Context()
	adminClient := sut.CreateWebClientForUser(t, "admin")
	endpoint := adminClient.Endpoint("enterprise", "accesslistpreset")

	aclWatcher := sut.NewResourceWatcher(t, types.KindAccessList)

	bobClient := sut.CreateWebClientForUser(t, "bob")
	resources := common.MustListUnifedResources(t, bobClient, common.WithSearchAsRole())
	require.Empty(t, resources.Items, "bob should have no searchable resources initially")

	req := buildAccessListWithPresetRequest()
	req.PresetType = "short-term"

	resp, err := common.Roundtrip[ui.AccessListWithPresetResponse](ctx, adminClient, http.MethodPost, endpoint, req)
	require.NoError(t, err)

	t.Run("bob can request JIT access to dev resources", func(t *testing.T) {
		bobClient := sut.CreateWebClientForUser(t, "bob")

		// Wait for user login state to be updated with requester role
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			uls, err := sut.Teleport.Process.GetAuthServer().Cache.GetUserLoginState(ctx, "bob")
			require.NoError(t, err)
			require.Len(t, uls.Spec.Roles, 2, "bob should have nop-role + requester role")
		}, time.Second*3, time.Millisecond*30)

		// Bob can search for resources via the requester role
		resources := common.MustListUnifedResources(t, bobClient, common.WithSearchAsRole())
		require.Len(t, resources.Items, 1, "bob should be able to search for 1 dev resource")

		// Bob creates an access request for the dev resource
		accessRequest, err := common.CreateAccessRequest(ctx, bobClient, ui.AccessRequestParameters{
			RequestKind: types.AccessRequestKind_SHORT_TERM,
			ResourceIDs: []ui.ResourceID{
				{
					Kind:        resources.Items[0].Kind,
					Name:        resources.Items[0].Name,
					ClusterName: "local-site",
				},
			},
		})
		require.NoError(t, err)

		// Alice (owner) approves the access request
		aliceClient := sut.CreateWebClientForUser(t, "alice")
		common.MustApproveAccessRequest(t, aliceClient, accessRequest.ID)

		// Bob still has no direct access to resources
		resources = common.MustListUnifedResources(t, bobClient)
		require.Empty(t, resources.Items, "bob should have no direct access without assuming the request")

		// Bob assumes the approved access request and gains JIT access
		bobJITClient, err := common.AssumeAccessRequestWebClient(ctx, bobClient, accessRequest.ID)
		require.NoError(t, err)
		resources = common.MustListUnifedResources(t, bobJITClient)
		require.Len(t, resources.Items, 1, "bob should have JIT access to 1 dev resource")
	})

	t.Run("update access list to add prod environment", func(t *testing.T) {

		// Wait for ineligibility reconciliation to complete before proceeding with the update.
		// This ensures the correct revision is used and prevents reconciliation from occurring
		// during the upgrade, which could lead to a revision mismatch.
		rev := waitForAccessListEligibilityReconciler(t, aclWatcher, resp.AccessList.GetName()).GetRevision()
		resp.AccessList.SetRevision(rev)

		req := ui.AccessListWithPresetRequest{
			PresetType: "short-term",
			AccessList: resp.AccessList,
			AccessRoles: []*types.RoleV6{
				resp.AccessRoles[0], // Keep existing dev role
				{
					Metadata: types.Metadata{Name: "prod"},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							AppLabels: types.Labels{"env": []string{"prod"}},
						},
					},
				},
			},
		}

		endpoint := adminClient.Endpoint("enterprise", "accesslistpreset", resp.AccessList.GetName())
		resp, err = common.Roundtrip[ui.AccessListWithPresetResponse](ctx, adminClient, http.MethodPut, endpoint, req)
		require.NoError(t, err)
		got := libslice.Map(resp.AccessRoles, func(t *types.RoleV6) string { return t.Metadata.Name })
		require.ElementsMatch(t, []string{"dev-acl-preset-dev-jit-access", "prod-acl-preset-dev-jit-access"}, got)

		// Bob can now search for both dev and prod resources
		bobClient := sut.CreateWebClientForUser(t, "bob")
		resources := common.MustListUnifedResources(t, bobClient, common.WithSearchAsRole())
		require.Len(t, resources.Items, 2, "bob should be able to search for dev and prod resources")

		req = ui.AccessListWithPresetRequest{
			PresetType: "short-term",
			AccessList: resp.AccessList,
			AccessRoles: []*types.RoleV6{
				resp.AccessRoles[0], // Keep existing dev role
			},
		}

		resp, err = common.Roundtrip[ui.AccessListWithPresetResponse](ctx, adminClient, http.MethodPut, endpoint, req)
		require.NoError(t, err)
		require.ElementsMatch(t, resp.RolesToBeDeleted, []string{"prod-acl-preset-dev-jit-access"})
	})
	mustDeleteAccessListAndRoles(t, sut, resp)
}

// waitForAccessListEligibilityReconciler waits until the named access list's user
// owners have a non-empty IneligibleStatus, and returns that observed
// resource.
// IneligibleStatusReconciler asynchronously sets the owners/members ineligibility status
// that bumps up the access list revision.
func waitForAccessListEligibilityReconciler(t *testing.T, watcher types.Watcher, name string) *accesslist.AccessList {
	t.Helper()
	return common.WaitForPutEvent(t, watcher, func(al *accesslist.AccessList) bool {
		if al.GetName() != name {
			return false
		}
		for _, owner := range al.Spec.Owners {
			if owner.IsMembershipKindUser() && owner.IneligibleStatus == "" {
				return false
			}
		}
		return true
	})
}

func mustDeleteAccessListAndRoles(t *testing.T, sut *common.SUT, resp ui.AccessListWithPresetResponse) {
	auth := sut.Teleport.Process.GetAuthServer()
	roles := make([]string, 0)

	for _, role := range resp.AccessRoles {
		roles = append(roles, role.Metadata.Name)
	}

	roles = append(roles, resp.AccessList.GetGrants().Roles...)
	roles = append(roles, resp.AccessList.GetOwnerGrants().Roles...)
	roles = append(roles, resp.RolesToBeDeleted...)

	err := auth.DeleteAccessList(t.Context(), resp.AccessList.GetName())
	require.NoError(t, err)

	for _, role := range slices.Compact(roles) {
		err := auth.DeleteRole(t.Context(), role)
		require.NoError(t, err)
	}
}
