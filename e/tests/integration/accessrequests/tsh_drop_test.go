package accessrequests

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/client"
	tshcommon "github.com/gravitational/teleport/tool/tsh/common"
)

// TestTSHRequestDropKeepsAccessListGrants validate the tsh access request
// drop flow for a user whose roles are  granted through an Access List
// are retained after dropping access requests.
func TestTSHRequestDropKeepsAccessListGrants(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "acl-granted"),
		common.WithRole(t, "direct-granted"),
		common.WithRole(t, "requester", func(r *types.RoleV6) {
			r.Spec.Allow.Request = &types.AccessRequestConditions{
				Roles:         []string{"access"},
				SearchAsRoles: []string{"access"},
			}
		}),
		common.WithUser(t, "bob", "requester"),
	)

	common.CreateAccessList(t, sut,
		common.WithName("bob-acl"),
		common.WithOwners("list-owner"),
		common.WithMembers("bob"),
		common.WithGrants(accesslist.Grants{Roles: []string{"acl-granted"}}),
	)

	authServer := sut.Teleport.Process.GetAuthServer()
	clusterName := sut.Teleport.Config.Auth.ClusterName.GetClusterName()
	node := mustCreateNode(t.Context(), t, authServer, "test-node", "test-node.example.com")

	password := uuid.NewString()
	require.NoError(t, authServer.UpsertPassword("bob", []byte(password)))

	tshHome := t.TempDir()
	runTSH := func(t *testing.T, args ...string) {
		t.Helper()
		err := tshcommon.Run(t.Context(), append(args, "--insecure"), func(cf *tshcommon.CLIConf) error {
			cf.HomePath = tshHome
			cf.TSHConfig = client.TSHConfig{}
			return nil
		})
		require.NoError(t, err)
	}
	readProfile := func(t *testing.T) *client.ProfileStatus {
		t.Helper()
		profile, err := client.NewFSClientStore(tshHome).ReadProfileStatus(sut.ProxyAddr)
		require.NoError(t, err)
		return profile
	}
	createApproveAssume := func(t *testing.T, createArgs ...string) string {
		t.Helper()
		runTSH(t, append([]string{"request", "create", "--nowait"}, createArgs...)...)

		requests, err := authServer.GetAccessRequests(t.Context(), types.AccessRequestFilter{User: "bob"})
		require.NoError(t, err)
		var requestID string
		for _, req := range requests {
			if req.GetState().IsPending() {
				require.Empty(t, requestID, "expected exactly one pending access request")
				requestID = req.GetName()
			}
		}
		require.NotEmpty(t, requestID)

		require.NoError(t, authServer.SetAccessRequestState(t.Context(), types.AccessRequestUpdate{
			RequestID: requestID,
			State:     types.RequestState_APPROVED,
		}))

		runTSH(t, "login", "--proxy="+sut.ProxyAddr, "--request-id="+requestID)
		return requestID
	}

	// Log in with tsh through the real local auth flow. The login hook
	// generates bob's user login state from the Access List membership.
	oldStdin := prompt.Stdin()
	t.Cleanup(func() { prompt.SetStdin(oldStdin) })
	prompt.SetStdin(prompt.NewFakeReader().AddString(password))
	runTSH(t, "login", "--proxy="+sut.ProxyAddr, "--auth=local", "--user=bob")

	// The login certs carry both the direct role and the Access List grant.
	profile := readProfile(t)
	require.ElementsMatch(t, []string{"requester", "acl-granted"}, profile.Roles)
	require.Empty(t, profile.ActiveRequests)

	t.Run("role access request", func(t *testing.T) {
		requestID := createApproveAssume(t, "--roles=access")

		profile := readProfile(t)
		require.ElementsMatch(t, []string{"requester", "acl-granted", "access"}, profile.Roles)
		require.Equal(t, []string{requestID}, profile.ActiveRequests)
		require.Empty(t, profile.AllowedResourceAccessIDs)

		// Dropping all requests must keep the Access List grant.
		runTSH(t, "request", "drop")

		profile = readProfile(t)
		require.ElementsMatch(t, []string{"requester", "acl-granted"}, profile.Roles)
		require.Empty(t, profile.ActiveRequests)
	})

	t.Run("resource access request", func(t *testing.T) {
		resourceID := types.ResourceIDToString(types.ResourceID{
			ClusterName: clusterName,
			Kind:        types.KindNode,
			Name:        node.GetName(),
		})
		requestID := createApproveAssume(t, "--resource="+resourceID)

		profile := readProfile(t)
		require.ElementsMatch(t, []string{"requester", "acl-granted", "access"}, profile.Roles)
		require.Equal(t, []string{requestID}, profile.ActiveRequests)
		require.Len(t, profile.AllowedResourceAccessIDs, 1)
		require.Equal(t, node.GetName(), profile.AllowedResourceAccessIDs[0].Id.Name)

		// Dropping the resource request must also keep the Access List grant.
		runTSH(t, "request", "drop", requestID)
		profile = readProfile(t)

		require.ElementsMatch(t, []string{"requester", "acl-granted"}, profile.Roles)
		require.Empty(t, profile.ActiveRequests)
		require.Empty(t, profile.AllowedResourceAccessIDs)
	})

	t.Run("backend user role added after login is picked up on drop", func(t *testing.T) {
		user, err := authServer.GetUser(t.Context(), "bob", false)
		require.NoError(t, err)
		user.AddRole("direct-granted")
		_, err = authServer.UpdateUser(t.Context(), user)
		require.NoError(t, err)

		runTSH(t, "request", "drop")

		profile := readProfile(t)
		require.ElementsMatch(t, []string{"requester", "acl-granted", "direct-granted"}, profile.Roles)
		require.Empty(t, profile.ActiveRequests)
	})
}
