package awsic

import (
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func requireTestClusterWithIdentityCenter(t *testing.T) (*common.SUT, *ictest.UnifiedClientMock) {
	ctx := t.Context()

	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.SetDefault(
		slog.New(logutils.NewSlogTextHandler(
			os.Stderr, logutils.SlogTextHandlerConfig{
				EnableColors: true,
				Level:        slog.LevelDebug,
			})))

	client := ictest.NewUnifiedMockClient(icsdk.NewMockedAWSState())
	mockIC := client.ViaAPI()
	mockSCIM := client.ViaSCIM()
	setupMockAWSICEnvironment(t, mockIC, mockSCIM)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice", "editor"),
	)

	auth := sut.Teleport.Process.GetAuthServer()
	mustCreateAWSICPlugin(t, sut.GetClusterClientForUser(t, "alice").AuthClient)

	// On the first run through, the underlying SCIM provisioner has to adopt
	// the downstream groups before the IC permissions calculator and provisioner
	// even get started on them. This can take a while in resource-constrained
	// environments (like the flaky test detector), so we need to give
	// this assertion a bit more time than the regular `require.Eventually()` 3
	// seconds

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			accounts, _, err := auth.ListIdentityCenterAccounts(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, mockIC.Accounts, len(accounts))

			permissionSet, _, err := auth.ListPermissionSets(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, mockIC.PermissionSets, len(permissionSet))

			accList, _, err := auth.ListAccessLists(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, mockIC.Groups, len(accList))
		},
		10*time.Second, 100*time.Millisecond)

	return sut, client
}

func TestAWSResourceSyncHandlesRenamedAccounts(t *testing.T) {
	ctx := t.Context()
	setAWSSyncInterval(t, time.Second)

	// GIVEN a running Teleport Cluster with a running Identity Center
	// integration...
	sut, mockIC := requireTestClusterWithIdentityCenter(t)

	// WHEN an AWS Account gets renamed outside of Teleport's control
	mockIC.Mu.Lock()
	modifiedAWSAccount := mockIC.Accounts[1]
	originalAWSAccount := *modifiedAWSAccount
	modifiedAWSAccount.Name = "modified_account"
	mockIC.Mu.Unlock()

	oldRoles := makeAccountRoleNames(mockIC.PermissionSets, &originalAWSAccount)
	newRoles := makeAccountRoleNames(mockIC.PermissionSets, modifiedAWSAccount)
	auth := sut.Teleport.Process.GetAuthServer()
	logScope := common.NewLogScope[*apievents.AWSICResourceSync](sut)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// EXPECT that the change is eventually propagated to Teleport
			requireICAccount(ctx, t, auth.IdentityCenter, modifiedAWSAccount.ID,
				withAccountName("modified_account"))

			// EXPECT that new roles referencing the updated account name have been created
			for _, newRole := range newRoles {
				requireRole(ctx, t, auth.Access, newRole,
					withRoleSubkind(types.KindIdentityCenter))
			}

			// EXPECT that the roles referencing the old account name have been deprecated
			for i, oldRole := range oldRoles {
				requireRole(ctx, t, auth.Access, oldRole,
					withRoleSubkind(""),
					withRoleLabel("teleport.internal/replaced_with", newRoles[i]))
			}
		},
		10*time.Second, 250*time.Millisecond)

	logScope.RequireEvent(t, events.AWSICResourceSyncSuccessEvent)

	// WHEN a previously-renamed AWS Account gets renamed back to its original
	// name
	logScope.Reset()
	mockIC.Mu.Lock()
	mockIC.Accounts[1].Name = originalAWSAccount.Name
	mockIC.Mu.Unlock()

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// EXPECT that the account name change is eventually propagated to
			// Teleport
			requireICAccount(ctx, t, auth.IdentityCenter, modifiedAWSAccount.ID,
				withAccountName(originalAWSAccount.Name))

			// EXPECT that roles referencing the original account name have been
			// re-adopted
			for _, oldRole := range oldRoles {
				requireRole(ctx, t, auth.Access, oldRole,
					withRoleSubkind(types.KindIdentityCenter))
			}

			// EXPECT that the roles referencing the modified account name have
			// been deprecated
			for i, newRole := range newRoles {
				requireRole(ctx, t, auth.Access, newRole,
					withRoleSubkind(""),
					withRoleLabel("teleport.internal/replaced_with", oldRoles[i]))
			}
		},
		10*time.Second, 250*time.Millisecond)
	logScope.RequireEvent(t, events.AWSICResourceSyncSuccessEvent)
}

func TestAWSResourceSyncHandlesRenamedPermissionSets(t *testing.T) {
	ctx := t.Context()
	setAWSSyncInterval(t, 5*time.Second)

	// GIVEN a running Teleport Cluster with a running Identity Center
	// integration...
	sut, mockIC := requireTestClusterWithIdentityCenter(t)

	// WHEN an AWS Account gets renamed outside of Teleport's control
	mockIC.Mu.Lock()
	modifiedPS := mockIC.PermissionSets[1]
	originalPS := *modifiedPS
	modifiedPS.Name = "modified_ps"
	mockIC.Mu.Unlock()

	oldRoles := makePermissionSetRoleNames(mockIC.Accounts, &originalPS)
	newRoles := makePermissionSetRoleNames(mockIC.Accounts, modifiedPS)
	auth := sut.Teleport.Process.GetAuthServer()

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// EXPECT that the change is eventually propagated to Teleport
			requirePermissionSet(ctx, t, auth.IdentityCenter, byPSARN(modifiedPS.ARN),
				withPSName("modified_ps"))

			// EXPECT that new roles referencing the updated account name have been created
			for _, newRole := range newRoles {
				requireRole(ctx, t, auth.Access, newRole,
					withRoleSubkind(types.KindIdentityCenter))
			}

			// EXPECT that the roles referencing the old account name have been deprecated
			for i, oldRole := range oldRoles {
				requireRole(ctx, t, auth.Access, oldRole,
					withRoleSubkind(""),
					withRoleLabel("teleport.internal/replaced_with", newRoles[i]))
			}
		},
		10*time.Second, 250*time.Millisecond)
}

func TestAWSResourceSyncHandlesDeletedAccounts(t *testing.T) {
	ctx := t.Context()
	setAWSSyncInterval(t, 5*time.Second)

	// GIVEN a running Teleport Cluster with a running Identity Center
	// integration...
	sut, mockIC := requireTestClusterWithIdentityCenter(t)

	// WHEN an AWS Account gets deleted outside of Teleport's control
	mockIC.Mu.Lock()
	deletedAWSAccount := mockIC.Accounts[1]
	mockIC.Accounts = slices.Delete(mockIC.Accounts, 1, 2)
	mockIC.Mu.Unlock()

	roles := makeAccountRoleNames(mockIC.PermissionSets, deletedAWSAccount)
	auth := sut.Teleport.Process.GetAuthServer()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// EXPECT that the change is eventually propagated to Teleport
			requireNoICAccount(ctx, t, auth.IdentityCenter, deletedAWSAccount.ID)

			// EXPECT that the roles referencing the deleted account have been
			// deprecated
			for _, role := range roles {
				requireRole(ctx, t, auth.Access, role,
					withRoleSubkind(""),
					withRoleLabel("teleport.internal/replaced_with", ""))
			}
		},
		10*time.Second, 250*time.Millisecond)
}

func TestAWSResourceSyncHandlesDeletedPermissionSets(t *testing.T) {
	ctx := t.Context()
	setAWSSyncInterval(t, 5*time.Second)

	// GIVEN a running Teleport Cluster with a running Identity Center
	// integration...
	sut, mockIC := requireTestClusterWithIdentityCenter(t)

	// WHEN an AWS Account gets renamed outside of Teleport's control
	mockIC.Mu.Lock()
	deletedPS := mockIC.PermissionSets[1]
	mockIC.PermissionSets = slices.Delete(mockIC.PermissionSets, 1, 2)
	mockIC.Mu.Unlock()

	oldRoles := makePermissionSetRoleNames(mockIC.Accounts, deletedPS)
	auth := sut.Teleport.Process.GetAuthServer()

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// EXPECT that the change is eventually propagated to Teleport and
			// our local PermissionSet record is deleted
			requireNoPermissionSet(ctx, t, auth.IdentityCenter, byPSARN(deletedPS.ARN))

			// EXPECT that the roles referencing the old account name have been deprecated
			for _, oldRole := range oldRoles {
				requireRole(ctx, t, auth.Access, oldRole,
					withRoleSubkind(""),
					withRoleLabel("teleport.internal/replaced_with", ""))
			}
		},
		10*time.Second, 250*time.Millisecond)
}

func setAWSSyncInterval(t *testing.T, d time.Duration) {
	oldSyncInterval := identitycenter.DefaultResourceSyncInterval
	identitycenter.DefaultResourceSyncInterval = d
	t.Cleanup(func() {
		identitycenter.DefaultResourceSyncInterval = oldSyncInterval
	})
}

func makeAccountRoleNames(pss []*icsdk.PermissionSet, acct *icsdk.Account) []string {
	return sliceutils.Map(pss, func(ps *icsdk.PermissionSet) string {
		return strings.ToLower(ps.Name + "-on-" + acct.Name + "-" + acct.ID)
	})
}

func makePermissionSetRoleNames(accts []*icsdk.Account, ps *icsdk.PermissionSet) []string {
	return sliceutils.Map(accts, func(acct *icsdk.Account) string {
		return strings.ToLower(ps.Name + "-on-" + acct.Name + "-" + acct.ID)
	})
}
