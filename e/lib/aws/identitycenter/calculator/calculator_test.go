package calculator

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/lib/services"
)

type mockExternalIDGetter struct {
	mu          sync.Mutex
	users       map[string]provisioning.ExternalID
	accessLists map[string]provisioning.ExternalID
}

func newMockExternalIDGetter() *mockExternalIDGetter {
	return &mockExternalIDGetter{
		users:       map[string]provisioning.ExternalID{},
		accessLists: map[string]provisioning.ExternalID{},
	}
}

func (m *mockExternalIDGetter) GetUserExternalID(_ context.Context, localID string) (provisioning.ExternalID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if extID, ok := m.users[localID]; ok {
		return extID, nil
	}
	return "", trace.NotFound(localID)
}

func (m *mockExternalIDGetter) GetAccessListExternalID(_ context.Context, localID string) (provisioning.ExternalID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if extID, ok := m.accessLists[localID]; ok {
		return extID, nil
	}
	return "", trace.NotFound(localID)
}

func (m *mockExternalIDGetter) setMockUser(t *testing.T, localID string, externalID provisioning.ExternalID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[localID] = externalID

	t.Cleanup(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.users, localID)
	})
}

func (m *mockExternalIDGetter) setMockAccessList(t *testing.T, localID string, externalID provisioning.ExternalID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessLists[localID] = externalID

	t.Cleanup(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.accessLists, localID)
	})
}

func TestAssignmentCalculation(t *testing.T) {
	externalIDs := newMockExternalIDGetter()
	logger := slog.Default().With("test", t.Name())
	fixture := ictest.NewFixture(t, ictest.WithCache(ictest.CacheArgs{Started: true}))
	ctx := fixture.Ctx

	calc, err := New(Config{
		AccessRequestsSvc:       fixture.Auth.Services,
		Clock:                   fixture.Clock,
		ExternalIDGetter:        externalIDs,
		PrincipalAssignmentsSvc: fixture.Auth.Services,
		Logger:                  logger,
		RolesGetter:             fixture.Auth.Services,
		AccountAssignmentCache:  fixture.Auth.Cache,
	})
	require.NoError(t, err)

	resources := makeTestResources(t, ctx, fixture)

	t.Run("User External ID set if empty", func(t *testing.T) {
		user, userPrincipal := makeTestUser(t, "paul@atreides.duchy.ar", "", fixture)

		externalIDs.setMockUser(t, user.GetName(), "EXTERNAL-ID")

		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)
		require.NoError(t, err)
		require.Equal(t, "EXTERNAL-ID", userPrincipal.Spec.ExternalId)
	})

	t.Run("User with no External ID is an error", func(t *testing.T) {
		user, userPrincipal := makeTestUser(t, "paul@atreides.duchy.ar", "", fixture)

		// The user has no External ID, and because we have not added one into
		// the mock ExternalID finder, so there is no way of finding one. Expect
		// the assignment calculation to fail.

		_, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)
		require.Error(t, err)

		// Now add an illegal empty external ID to the mock ExternalID finder,
		// in order to assert that the calculation fails gracefully in the face
		// of bad data
		externalIDs.setMockUser(t, user.GetName(), "")

		_, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)
		require.Error(t, err)
	})

	t.Run("Access List External ID set if empty", func(t *testing.T) {
		owner, _ := makeTestUser(t, "emperor@corrino.imperium.ka", "", fixture)
		acl, aclPrincipal := makeTestAccessList(t, "acl-one", owner, nil, "", fixture)

		externalIDs.setMockAccessList(t, acl.GetName(), "LIST-#1-EXTERNAL-ID")

		aclPrincipal, err = calc.calcAccessListAssignments(ctx, acl, aclPrincipal)
		require.NoError(t, err)
		require.Equal(t,
			provisioning.ExternalID("LIST-#1-EXTERNAL-ID"),
			principal.GetExternalID(aclPrincipal))
	})

	t.Run("Access List with no External ID is an error", func(t *testing.T) {
		owner, _ := makeTestUser(t, "emperor@corrino.imperium.ka", "", fixture)
		acl, aclPrincipal := makeTestAccessList(t, "acl-one", owner, nil, "", fixture)

		// The Access List has no External ID, and because we have not added one
		// into he mock ExternalID finder, so there is no way of finding one for
		// it. Expect the assignment calculation to fail.

		_, err = calc.calcAccessListAssignments(ctx, acl, aclPrincipal)
		require.Error(t, err)

		// add an empty external ID to the mock ExternalID finder, so that we
		// can check that the calculation fails gracefully in the face of bad
		// data
		externalIDs.setMockAccessList(t, acl.GetName(), "")

		_, err = calc.calcAccessListAssignments(ctx, acl, aclPrincipal)
		require.Error(t, err)
	})

	t.Run("Access List Roles Allow", func(t *testing.T) {
		// GIVEN an access list with multiple account-assignment roles
		owner, _ := makeTestUser(t, "emperor@corrino.imperium.ka", "", fixture)
		acl, aclPrincipal := makeTestAccessList(t, "acl-one", owner, nil, "ACL-EXTERNAL-ID", fixture)

		assignments := []assignment{
			{
				accountID:        resources.accounts[3].Metadata.Name,
				permissionSetARN: resources.permissonSets[2].Spec.Arn,
			},
			{
				accountID:        resources.accounts[2].Metadata.Name,
				permissionSetARN: resources.permissonSets[1].Spec.Arn,
			},
			{
				accountID:        resources.accounts[1].Metadata.Name,
				permissionSetARN: resources.permissonSets[2].Spec.Arn,
			},
			{
				accountID:        resources.accounts[0].Metadata.Name,
				permissionSetARN: resources.permissonSets[2].Spec.Arn,
			},
		}

		for _, key := range assignments {
			acl.Spec.Grants.Roles = append(acl.Spec.Grants.Roles,
				resources.roles[key].GetName())
		}

		// WHEN I attempt to calculate the Access List's assignment set
		aclPrincipal, err = calc.calcAccessListAssignments(ctx, acl, aclPrincipal)

		// EXPECT the operation to succeed, and the provisioning state to be set
		// to STALE to be picked up by the assignment provisioner
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			aclPrincipal.Status.ProvisioningState)

		// EXPECT that the assignments reflected in the principal state are what
		// we expect.
		requireAssignmentsMatch(t, assignments, aclPrincipal)
	})

	t.Run("User Roles Allow", func(t *testing.T) {
		user, userPrincipal := makeTestUser(t, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID", fixture)

		assignments := []assignment{
			{
				accountID:        resources.accounts[1].Metadata.Name,
				permissionSetARN: resources.permissonSets[0].Spec.Arn,
			},
			{
				accountID:        resources.accounts[3].Metadata.Name,
				permissionSetARN: resources.permissonSets[2].Spec.Arn,
			},
			{
				accountID:        resources.accounts[2].Metadata.Name,
				permissionSetARN: resources.permissonSets[1].Spec.Arn,
			},
		}

		for _, key := range assignments {
			user.AddRole(resources.roles[key].GetName())
		}

		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		var actualAssignments []assignment
		for _, asmt := range userPrincipal.Status.Assignments {
			actualAssignments = append(actualAssignments, assignment{
				accountID:        asmt.AccountId,
				permissionSetARN: asmt.PermissionSetArn,
			})
		}
		require.ElementsMatch(t, assignments, actualAssignments)
	})

	t.Run("User Roles Allow with PS Glob", func(t *testing.T) {
		// GIVEN a role that allows all PermissionSets in Account #01
		roleSpec := types.RoleSpecV6{
			Allow: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       resources.accounts[1].Metadata.Name,
						PermissionSet: "*",
					},
				},
			},
		}
		allowRole := makeTestRole(t, "allow-all-on-acct1", roleSpec, fixture)

		// GIVEN a user that has that allow role
		user, userPrincipal := makeTestUser(t, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID", fixture)
		user.AddRole(allowRole.GetName())

		// WHEN I attempt to calculate the user assignment set
		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)

		// EXPECT the operation to succeed, and the state to marked as STALE
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		// EXPECT the calculated assignments to include all of the permission
		// sets in Account #01, and nothing else
		expected := slices.Collect(maps.Keys(resources.accountAssignments))
		expected = slices.DeleteFunc(expected, func(a assignment) bool {
			return a.accountID != resources.accounts[1].Metadata.Name
		})
		requireAssignmentsMatch(t, expected, userPrincipal)
	})

	t.Run("User Roles Deny with PS Glob", func(t *testing.T) {
		// GIVEN a role that denies access to all permission sets on Account #02
		roleSpec := types.RoleSpecV6{
			Deny: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       resources.accounts[2].Metadata.Name,
						PermissionSet: "*",
					},
				},
			},
		}
		denyRole := makeTestRole(t, "deny_deny_deny", roleSpec, fixture)

		// GIVEN a user that has all known account assignment roles, AND the
		// deny-all-on-account2 role
		user, userPrincipal := makeTestUser(t, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID", fixture)
		for _, role := range resources.roles {
			user.AddRole(role.GetName())
		}
		user.AddRole(denyRole.GetName())

		// WHEN I calculate the user's assignments
		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)

		// EXPECT that the operation succedeed and the user has been marked
		// stale for provisioning
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		// EXPECT that the calculated permission set contains only the assignments
		// NOT for Accout #2

		// Generate the expected assignment list - all assignments minus those
		// for Account #2
		expected := slices.Collect(maps.Keys(resources.accountAssignments))
		expected = slices.DeleteFunc(expected, func(a assignment) bool {
			return a.accountID == resources.accounts[2].Metadata.Name
		})
		requireAssignmentsMatch(t, expected, userPrincipal)
	})
}

// requireAssignmentsMatch asserts that the assignments listed in the principal
// assignment record match the expected set of assignments. Assignment ordering
// is considered irrelevant
func requireAssignmentsMatch(t *testing.T, expected []assignment, principalAssignment *identitycenterv1.PrincipalAssignment) {
	t.Helper()

	// Step 1: Pack the calculated assignment list into an easy-to-compare form
	var actualAssignments []assignment
	for _, asmt := range principalAssignment.Status.Assignments {
		actualAssignments = append(actualAssignments, assignment{
			accountID:        asmt.AccountId,
			permissionSetARN: asmt.PermissionSetArn,
		})
	}

	// Step 2: do the comparison
	require.ElementsMatch(t, expected, actualAssignments)
}

// makeTestAccessList creates a test Access List and corresponding Principal
// Assignment that are automatically deleted at the end of the test
func makeTestAccessList(t *testing.T,
	name string,
	owner types.User,
	memberGrants []types.Role,
	externalID provisioning.ExternalID,
	fixture *ictest.Fixture,
) (*accesslist.AccessList, *identitycenterv1.PrincipalAssignment) {
	acl := ictest.AccessList{
		Name:          name,
		Title:         "Access list " + name,
		GrantsMembers: memberGrants,
		Owners:        []types.User{owner},
	}.Build(t)

	acl, err := fixture.Auth.UpsertAccessList(fixture.Ctx, acl)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := fixture.Auth.DeleteAccessList(fixture.Ctx, acl.GetName())
		require.NoError(t, err)
	})

	aclPrincipal, err := principal.NewFor(acl)
	require.NoError(t, err)
	aclPrincipal.Spec.ExternalId = string(externalID)
	aclPrincipal, err = fixture.Auth.CreatePrincipalAssignment(fixture.Ctx, aclPrincipal)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := fixture.Auth.DeletePrincipalAssignment(fixture.Ctx, principal.GetID(aclPrincipal))
		require.NoError(t, err)
	})

	return acl, aclPrincipal
}

// makeTestUser creates a test User and corresponding Principal Assignment that
// are automatically deleted at the end of the test
func makeTestUser(
	t *testing.T,
	name string,
	externalID provisioning.ExternalID,
	fixture *ictest.Fixture,
) (types.User, *identitycenterv1.PrincipalAssignment) {
	user, err := types.NewUser(name)
	require.NoError(t, err)
	user, err = fixture.Auth.CreateUser(fixture.Ctx, user)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := fixture.Auth.DeleteUser(fixture.Ctx, user.GetName())
		require.NoError(t, err)
	})

	userPrincipal, err := principal.NewFor(user)
	require.NoError(t, err)
	userPrincipal.Spec.ExternalId = string(externalID)
	userPrincipal, err = fixture.Auth.CreatePrincipalAssignment(fixture.Ctx, userPrincipal)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := fixture.Auth.DeletePrincipalAssignment(fixture.Ctx, principal.GetID(userPrincipal))
		require.NoError(t, err)
	})

	return user, userPrincipal
}

// makeTestRole creates a test Role that is automatically deleted at the end of
// the test
func makeTestRole(t *testing.T, name string, spec types.RoleSpecV6, fixture *ictest.Fixture) types.Role {
	role, err := types.NewRole(name, spec)
	require.NoError(t, err)
	role, err = fixture.Auth.CreateRole(fixture.Ctx, role)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := fixture.Auth.DeleteRole(fixture.Ctx, role.GetName())
		require.NoError(t, err)
	})
	return role
}

type testResources struct {
	accounts           []services.IdentityCenterAccount
	permissonSets      []*identitycenterv1.PermissionSet
	accountAssignments map[assignment]services.IdentityCenterAccountAssignment
	roles              map[assignment]types.Role
}

func makeTestResources(t *testing.T, ctx context.Context, fixture *ictest.Fixture) testResources {
	var err error
	var permissionSets []*identitycenterv1.PermissionSet
	for _, name := range []string{"Admin", "Readonly", "SecurityAudit"} {
		ps := ictest.PermissionSet{
			ID:          fmt.Sprintf("permissionset_%s", strings.ToLower(name)),
			Name:        name,
			Description: fmt.Sprintf("%s Permissions", name),
			ARN:         fmt.Sprintf("arn:aws:sso:::permissionSet/%s", name),
		}.Build()
		ps, err = fixture.Auth.CreatePermissionSet(ctx, ps)
		require.NoError(t, err)
		permissionSets = append(permissionSets, ps)
	}

	accounts := make([]services.IdentityCenterAccount, 4)
	accountAssignments := make(map[assignment]services.IdentityCenterAccountAssignment)
	accountAssignmentRoles := make(map[assignment]types.Role)
	for i := range accounts {
		accountID := strings.Repeat(strconv.Itoa(i), 8)
		account := ictest.Account{
			ID:             services.IdentityCenterAccountID(accountID),
			Name:           fmt.Sprintf("Account #%02d", i),
			ARN:            fmt.Sprintf("arn:aws:iam::%s:account/Account%02d", accountID, i),
			PermissionSets: slices.Values(permissionSets),
		}.Build()
		account, err = fixture.Auth.CreateIdentityCenterAccount(ctx, account)
		require.NoError(t, err)
		accounts[i] = account

		for _, ps := range permissionSets {
			key := assignment{accountID: accountID, permissionSetARN: ps.Spec.Arn}

			assignment := ictest.AccountAssignment{
				ID:                fmt.Sprintf("%s--%s", accountID, ps.Metadata.Name),
				DisplayName:       fmt.Sprintf("%s on %s", ps.Spec.Name, account.Spec.Name),
				AccountID:         services.IdentityCenterAccountID(accountID),
				PermissionSetName: ps.Spec.Name,
				PermissionSetARN:  ps.Spec.Arn,
			}.Build()
			assignment, err = fixture.Auth.CreateAccountAssignment(ctx, assignment)
			require.NoError(t, err)
			accountAssignments[key] = assignment

			role, err := fixture.Auth.Services.Access.CreateRole(ctx,
				ictest.AccountAssignmentRole{
					Name:             fmt.Sprintf("%s-on-%s", ps.Spec.Name, account.Spec.Name),
					AccountID:        services.IdentityCenterAccountID(accountID),
					PermissionSetARN: ps.Spec.Arn,
				}.Build(t))
			require.NoError(t, err)
			accountAssignmentRoles[key] = role
		}

		cachePopulated := func(c *assert.CollectT) {
			assertSequenceLength(c, len(accountAssignments), iciter.AllAccountAssignments(ctx, fixture.Auth.Cache))
		}
		require.EventuallyWithT(t, cachePopulated, time.Second, 10*time.Millisecond)
	}

	return testResources{
		accounts:           accounts,
		permissonSets:      permissionSets,
		accountAssignments: accountAssignments,
		roles:              accountAssignmentRoles,
	}
}

// assertSequenceLength asserts that a sequence can be read start-to-finish
// without error, and that the sequence has a specific length.
func assertSequenceLength[T any](t assert.TestingT, expectedLength int, seq iter.Seq2[T, error]) {
	length := 0
	for _, err := range seq {
		assert.NoError(t, err)
		if err != nil {
			return
		}
		length++
	}
	assert.Equal(t, expectedLength, length)
}
