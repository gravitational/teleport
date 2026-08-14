package main_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	authe "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestAWSICList(t *testing.T) {
	t.Parallel()

	client := setupAWSICSuite(t)

	out, err := runAWSICCommand(t, client, []string{"accounts", "ls"})
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "111111111111")
	assert.Contains(t, text, "Production")
	assert.Contains(t, text, "arn:aws:sso:::permissionSet/admin")
	assert.Contains(t, text, "arn:aws:sso:::permissionSet/readonly")
}

func setupAWSICSuite(t *testing.T) *authclient.Client {
	t.Helper()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			testModules := &modulestest.Modules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.Identity: {Enabled: true},
					},
				},
			}

			cfg.PluginRegistry = plugin.NewRegistry()
			cfg.Modules = testModules
			authPlugin, err := authe.NewPlugin(authe.Config{
				License: authe.ValidLicense{},
				Modules: testModules,
			})
			require.NoError(t, err)
			require.NoError(t, cfg.PluginRegistry.Add(authPlugin))
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	authServer := process.GetAuthServer()

	role, err := types.NewRole("ic-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindApp, []string{types.VerbList, types.VerbRead}),
				types.NewRule(types.KindIdentityCenterAccountAssignment, []string{types.VerbList, types.VerbRead}),
			},
			AccountAssignments: []types.IdentityCenterAccountAssignment{
				{Account: types.Wildcard, PermissionSet: types.Wildcard},
			},
		},
	})
	require.NoError(t, err)
	_, err = authServer.CreateRole(t.Context(), role)
	require.NoError(t, err)

	user, err := types.NewUser("admin")
	require.NoError(t, err)
	user.SetRoles([]string{role.GetName()})
	_, err = authServer.CreateUser(t.Context(), user)
	require.NoError(t, err)

	createICAccountAssignments(t, authServer, "111111111111", "Production")

	return makeClient(t, process, user.GetName())
}

func createICAccountAssignments(t *testing.T, authServer *auth.Server, accountID, name string) {
	t.Helper()

	permissionSets := []*identitycenterv1.PermissionSetInfo{
		identitycenterv1.PermissionSetInfo_builder{
			Name: "AdminAccess",
			Arn:  "arn:aws:sso:::permissionSet/admin",
		}.Build(),
		identitycenterv1.PermissionSetInfo_builder{
			Name: "ReadOnly",
			Arn:  "arn:aws:sso:::permissionSet/readonly",
		}.Build(),
	}

	for _, ps := range permissionSets {
		_, err := authServer.Services.CreateIdentityCenterAccountAssignment(t.Context(), identitycenterv1.AccountAssignment_builder{
			Kind:    types.KindIdentityCenterAccountAssignment,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: fmt.Sprintf("%s--%s", accountID, ps.GetName()),
				Labels: map[string]string{
					types.OriginLabel:         common.OriginAWSIdentityCenter,
					types.AWSAccountIDLabel:   accountID,
					types.AWSAccountNameLabel: name,
				},
			}.Build(),
			Spec: identitycenterv1.AccountAssignmentSpec_builder{
				Display:       fmt.Sprintf("%q on %q", ps.GetName(), name),
				PermissionSet: ps,
				AccountName:   name,
				AccountId:     accountID,
			}.Build(),
		}.Build())
		require.NoError(t, err)
	}
}
