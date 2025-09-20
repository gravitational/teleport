package awsic

import (
	"testing"
	"time"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestLockHandling(t *testing.T) {
	client := ictest.NewUnifiedMockClient(icsdk.NewMockedAWSState())
	setupMockAWSICEnvironment(t, client.ViaAPI(), client.ViaSCIM())

	ctx := t.Context()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "aws-ic-access", common.WithAccountAssignment(types.Allow, "*", "*")),
		common.WithUser(t, "alice", "editor"),
		common.WithUser(t, "bob", "requester"),
	)

	aliceClient := sut.GetClusterClientForUser(t, "alice")
	auth := sut.Teleport.Process.GetAuthServer()
	mustSetupAWSIdentityCenterIntegration(t, aliceClient.AuthClient)

	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			requireSCIMUsersExist(ctx, c, client.ViaSCIM(), "alice", "bob")
			requireRole(ctx, c, auth, "admin-on-account1-1111111111",
				hasAllowAccountAssignments(
					types.IdentityCenterAccountAssignment{
						Account:       "1111111111",
						PermissionSet: "arn:aws:sso:::permissionSet/Admin",
					}))
		},
		// Initial Identity Center startup takes a while to run, especially under
		// the flakey test detector, so we give this more than the usual 3s to run
		10*time.Second, 100*time.Millisecond,
		"Initial state setup failed or timed out")

	// WHEN I grant Bob an account assignment via a role
	mustUpdateUser(ctx, t, auth, "bob",
		func(u types.User) {
			u.AddRole("admin-on-account1-1111111111")
			u.AddRole("admin-on-account2-2222222222")
		})

	// EXPECT that the corresponding Account Assignments are created in AWS
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			requireICUser(ctx, c, client.ViaAPI(), "bob",
				hasAccountAssignments(
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "2222222222",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					}))
		},
		time.Second*3, time.Millisecond*30)

	// WHEN I lock the role...
	lock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: "admin-on-account2-2222222222"},
	})
	require.NoError(t, err)
	require.NoError(t, auth.UpsertLock(ctx, lock))

	// EXPECT that the role-granted Account Assignments from the locked role
	// have been removed from AWS
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			assertICUser(ctx, c, client.ViaAPI(), "bob",
				hasAccountAssignments(&icsdk.Assignment{
					AccountID:        "1111111111",
					PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
					PrincipalType:    ssoadmintypes.PrincipalTypeUser,
				}))
		},
		time.Second*3, time.Millisecond*30)

	// WHEN I create an approved Access Request that grants Bob some more roles
	roleAR, err := types.NewAccessRequest(uuid.NewString(), "bob",
		"readonly-on-account1-1111111111",
		"readonly-on-account2-2222222222")
	roleAR.SetExpiry(sut.Clock.Now().Add(7 * 24 * time.Hour))
	roleAR.SetAccessExpiry(sut.Clock.Now().Add(14 * 24 * time.Hour))
	require.NoError(t, err)
	require.NoError(t, roleAR.SetState(types.RequestState_APPROVED), "Must set state to APPROVED")
	require.NoError(t, auth.UpsertAccessRequest(ctx, roleAR))

	// EXPECT that the roles granted by the access request have been
	// reflected in AWS, in addition to Bob's standing permissions
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			assertICUser(ctx, c, client.ViaAPI(), "bob",
				hasAccountAssignments(
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "2222222222",
						PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
				))
		},
		time.Second*3, time.Millisecond*30)

	// WHEN I lock one role granted by the access request
	arRoleLock, err := types.NewLock("ar-role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: "readonly-on-account2-2222222222"},
	})
	require.NoError(t, err)
	require.NoError(t, auth.UpsertLock(ctx, arRoleLock))

	// EXPECT that the locked role is absent from the Access-Request granted
	// roles, but all other grants are preserved
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			assertICUser(ctx, c, client.ViaAPI(), "bob",
				hasAccountAssignments(
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
				))
		},
		time.Second*3, time.Millisecond*30)

	// WHEN I lock the whole Access Request
	arLock, err := types.NewLock("ar-lock", types.LockSpecV2{
		Target: types.LockTarget{AccessRequest: roleAR.GetName()},
	})
	require.NoError(t, err)
	require.NoError(t, auth.UpsertLock(ctx, arLock))

	// EXPECT that all Account Assignments granted by the Access request are
	// removed from Bob's AWS account
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			assertICUser(ctx, c, client.ViaAPI(), "bob",
				hasAccountAssignments(
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
				))
		},
		time.Second*3, time.Millisecond*30)

	// WHEN I delete all locks...
	require.NoError(t, auth.DeleteLock(ctx, lock.GetName()))
	require.NoError(t, auth.DeleteLock(ctx, arRoleLock.GetName()))
	require.NoError(t, auth.DeleteLock(ctx, arLock.GetName()))

	// EXPECT that all standing permissions and Access Request granted permissions
	// have been restored in AWS
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			assertICUser(ctx, c, client.ViaAPI(), "bob",
				hasAccountAssignments(
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "2222222222",
						PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "1111111111",
						PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
					&icsdk.Assignment{
						AccountID:        "2222222222",
						PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
						PrincipalType:    ssoadmintypes.PrincipalTypeUser,
					},
				))
		},
		time.Second*3, time.Millisecond*30)
}
