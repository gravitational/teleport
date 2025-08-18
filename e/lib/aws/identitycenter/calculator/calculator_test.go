package calculator

import (
	"context"
	"fmt"
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
	return "", trace.NotFound("%s", localID)
}

func (m *mockExternalIDGetter) GetAccessListExternalID(_ context.Context, localID string) (provisioning.ExternalID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if extID, ok := m.accessLists[localID]; ok {
		return extID, nil
	}
	return "", trace.NotFound("%s", localID)
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
	fixture := ictest.NewFixture(t, ictest.WithStartedCache)
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

	icAccessRole := []types.Role{
		makeTestRole(t, fixture, "test-aws-ic-access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       types.Wildcard,
						PermissionSet: types.Wildcard,
					},
				},
			},
		}),
	}

	t.Run("User External ID set if empty", func(t *testing.T) {
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "")

		externalIDs.setMockUser(t, user.GetName(), "EXTERNAL-ID")

		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)
		require.NoError(t, err)
		require.Equal(t, "EXTERNAL-ID", userPrincipal.Spec.ExternalId)
	})

	t.Run("User with no External ID is an error", func(t *testing.T) {
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "")

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
		owner, _ := makeTestUser(t, fixture, "emperor@corrino.imperium.ka", "")
		acl, aclPrincipal := makeTestAccessList(t, fixture, "acl-one", owner, nil, "")

		externalIDs.setMockAccessList(t, acl.GetName(), "LIST-#1-EXTERNAL-ID")

		aclPrincipal, err = calc.calcAccessListAssignments(ctx, acl, aclPrincipal)
		require.NoError(t, err)
		require.Equal(t,
			provisioning.ExternalID("LIST-#1-EXTERNAL-ID"),
			principal.GetExternalID(aclPrincipal))
	})

	t.Run("Access List with no External ID is an error", func(t *testing.T) {
		owner, _ := makeTestUser(t, fixture, "emperor@corrino.imperium.ka", "")
		acl, aclPrincipal := makeTestAccessList(t, fixture, "acl-one", owner, nil, "")

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
		owner, _ := makeTestUser(t, fixture, "emperor@corrino.imperium.ka", "")
		acl, aclPrincipal := makeTestAccessList(t, fixture, "acl-one", owner, nil, "ACL-EXTERNAL-ID")

		assignments := []index{
			{account: 3, ps: 2},
			{account: 2, ps: 1},
			{account: 1, ps: 2},
			{account: 0, ps: 2},
		}
		for _, i := range assignments {
			acl.Spec.Grants.Roles = append(acl.Spec.Grants.Roles,
				resources.getRole(i).GetName())
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
		expectedAssignments := resources.getAssignments(assignments...)
		requireAssignmentsMatch(t, expectedAssignments, aclPrincipal)
	})

	t.Run("User Roles Allow", func(t *testing.T) {
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID")

		assignments := []index{
			{account: 1, ps: 0},
			{account: 3, ps: 2},
			{account: 2, ps: 1},
		}
		for _, i := range assignments {
			user.AddRole(resources.getRole(i).GetName())
		}

		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		expectedAssignments := resources.getAssignments(assignments...)
		requireAssignmentsMatch(t, expectedAssignments, userPrincipal)
	})

	t.Run("User Roles Allow with PS Glob", func(t *testing.T) {
		// GIVEN a role that allows all PermissionSets in Account #01
		allowRole := makeTestRole(t, fixture, "allow-all-on-acct1", types.RoleSpecV6{
			Allow: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       resources.accounts[1].Metadata.Name,
						PermissionSet: "*",
					},
				},
			},
		})

		// GIVEN a user that has that allow role
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID")
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
		denyRole := makeTestRole(t, fixture, "deny_deny_deny", types.RoleSpecV6{
			Deny: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       resources.accounts[2].Metadata.Name,
						PermissionSet: "*",
					},
				},
			},
		})

		// GIVEN a user that has all known account assignment roles, AND the
		// deny-all-on-account2 role
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID")
		for _, role := range resources.roles {
			user.AddRole(role.GetName())
		}
		user.AddRole(denyRole.GetName())

		// WHEN I calculate the user's assignments
		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)

		// EXPECT that the operation succeeded and the user has been marked
		// stale for provisioning
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		// EXPECT that the calculated permission set contains only the assignments
		// NOT for Account #2

		// Generate the expected assignment list - all assignments minus those
		// for Account #2
		expected := slices.Collect(maps.Keys(resources.accountAssignments))
		expected = slices.DeleteFunc(expected, func(a assignment) bool {
			return a.accountID == resources.accounts[2].Metadata.Name
		})
		requireAssignmentsMatch(t, expected, userPrincipal)
	})

	t.Run("Role access requests are honored", func(t *testing.T) {
		// GIVEN a user
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID")

		// GIVEN a set of desired assignments
		assignments := []index{
			{account: 0, ps: 2},
			{account: 3, ps: 0},
		}

		// GIVEN an APPROVED access request granting access roles that grant
		// the requested assignments
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:   user,
			Expiry: fixture.Clock.Now().Add(time.Hour),
			Roles:  resources.getRoles(assignments...),
			State:  types.RequestState_APPROVED,
		}.Build(t))

		// GIVEN a PENDING access request granting access to a specific
		// role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:   user,
			Expiry: fixture.Clock.Now().Add(time.Hour),
			Roles:  resources.getRoles(index{account: 1, ps: 0}),
			State:  types.RequestState_PENDING,
		}.Build(t))

		// GIVEN a DENIED access request that is otherwise valid, which grants
		// a specific role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:   user,
			Expiry: fixture.Clock.Now().Add(time.Hour),
			Roles:  resources.getRoles(index{account: 1, ps: 1}),
			State:  types.RequestState_DENIED,
		}.Build(t))

		// GIVEN an APPROVED access request WITH A START TIME IN THE FUTURE that
		// grants a specific role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:            user,
			AssumeStartTime: fixture.Clock.Now().Add(30 * time.Minute),
			Expiry:          fixture.Clock.Now().Add(time.Hour),
			Roles:           resources.getRoles(index{account: 1, ps: 2}),
			State:           types.RequestState_APPROVED,
		}.Build(t))

		// GIVEN an APPROVED access request WITH AN EXPIRY TIME IN THE PAST
		// that grants a specific role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:   user,
			Expiry: fixture.Clock.Now().Add(-time.Hour),
			Roles:  resources.getRoles(index{account: 2, ps: 0}),
			State:  types.RequestState_APPROVED,
		}.Build(t))

		// WHEN I calculate the user's assignments
		var err error
		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)

		// EXPECT that the operation succeeded and the user has been marked
		// stale for provisioning
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		// EXPECT that the user has only the assignments granted by the approved,
		// in-window access request.
		expectedAssignments := resources.getAssignments(assignments...)
		requireAssignmentsMatch(t, expectedAssignments, userPrincipal)
	})

	t.Run("Resource access requests are honored", func(t *testing.T) {
		// GIVEN a user
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID")

		// GIVEN a set of desired assignments
		assignments := []index{
			{account: 0, ps: 2},
			{account: 3, ps: 0},
		}

		// GIVEN an APPROVED access request granting access to the desired Account
		// Assignment resources
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:        user,
			Expiry:      fixture.Clock.Now().Add(time.Hour),
			Roles:       icAccessRole,
			ResourceIDs: resources.getResourceIDs(assignments...),
			State:       types.RequestState_APPROVED,
		}.Build(t))

		// GIVEN a PENDING access request granting access to a specific
		// resource NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:        user,
			Expiry:      fixture.Clock.Now().Add(time.Hour),
			Roles:       icAccessRole,
			ResourceIDs: resources.getResourceIDs(index{account: 0, ps: 0}),
			State:       types.RequestState_PENDING,
		}.Build(t))

		// GIVEN a DENIED access request that is otherwise valid, which grants
		// a specific role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:        user,
			Expiry:      fixture.Clock.Now().Add(time.Hour),
			Roles:       icAccessRole,
			ResourceIDs: resources.getResourceIDs(index{account: 0, ps: 1}),
			State:       types.RequestState_DENIED,
		}.Build(t))

		// GIVEN an APPROVED access request WITH A START TIME IN THE FUTURE that
		// grants a specific role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:            user,
			AssumeStartTime: fixture.Clock.Now().Add(30 * time.Minute),
			Expiry:          fixture.Clock.Now().Add(time.Hour),
			Roles:           icAccessRole,
			ResourceIDs:     resources.getResourceIDs(index{account: 1, ps: 0}),
			State:           types.RequestState_APPROVED,
		}.Build(t))

		// GIVEN an APPROVED access request WITH AN EXPIRY TIME IN THE PAST
		// that grants a specific role NOT GRANTED BY ANY OTHER Access requests
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:        user,
			Expiry:      fixture.Clock.Now().Add(-time.Hour),
			Roles:       icAccessRole,
			ResourceIDs: resources.getResourceIDs(index{account: 1, ps: 1}),
			State:       types.RequestState_APPROVED,
		}.Build(t))

		// WHEN I calculate the user's assignments
		var err error
		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)

		// EXPECT that the operation succeeded and the user has been marked
		// stale for provisioning
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		// EXPECT that the user has only the assignments granted by the approved,
		// in-window access request.
		expectedAssignments := resources.getAssignments(assignments...)
		requireAssignmentsMatch(t, expectedAssignments, userPrincipal)
	})

	// This test assert that deny conditions on roles take precedence over access
	// requests grants
	t.Run("Role Deny conditions beat Resource Access Requests", func(t *testing.T) {
		// GIVEN a role that denies all access to account assignments on Account #0
		denyRole := makeTestRole(t, fixture, "deny_deny_deny", types.RoleSpecV6{
			Deny: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       resources.accounts[0].Metadata.Name,
						PermissionSet: "*",
					},
				},
			},
		})

		// GIVEN a user with that deny role
		user, userPrincipal := makeTestUser(t, fixture, "paul@atreides.duchy.ar", "MY-EXTERNAL-ID")
		user.AddRole(denyRole.GetName())

		// GIVEN an APPROVED access request granting access to some Account
		// Assignment resources
		makeTestAccessRequest(t, fixture, ictest.AccessRequest{
			User:   user,
			Expiry: fixture.Clock.Now().Add(time.Hour),
			Roles:  icAccessRole,
			ResourceIDs: resources.getResourceIDs(
				index{account: 0, ps: 2},
				index{account: 3, ps: 0},
			),
			State: types.RequestState_APPROVED,
		}.Build(t))

		// WHEN I calculate the user's assignments
		var err error
		userPrincipal, err = calc.calcUserAssignments(ctx, user.(*types.UserV2), userPrincipal)

		// EXPECT that the operation succeeded and the user has been marked
		// stale for provisioning
		require.NoError(t, err)
		require.Equal(t,
			identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			userPrincipal.Status.ProvisioningState)

		// EXPECT that the user has only the assignment granted by the access
		// request MINUS those denied by the `denied` role
		expectedAssignments := resources.getAssignments(index{account: 3, ps: 0})
		requireAssignmentsMatch(t, expectedAssignments, userPrincipal)
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
	fixture *ictest.Fixture,
	name string,
	owner types.User,
	memberGrants []types.Role,
	externalID provisioning.ExternalID,
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
	fixture *ictest.Fixture,
	name string,
	externalID provisioning.ExternalID,
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
func makeTestRole(t *testing.T, fixture *ictest.Fixture, name string, spec types.RoleSpecV6) types.Role {
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

// makeTestAccessRequest creates an Access Request that is automatically deleted
// at the end of the test.
func makeTestAccessRequest(t *testing.T, fixture *ictest.Fixture, req types.AccessRequest) {
	t.Helper()
	ctx := fixture.Ctx
	require.NoError(t, fixture.Auth.UpsertAccessRequest(ctx, req))
	t.Cleanup(func() {
		require.NoError(t, fixture.Auth.DeleteAccessRequest(ctx, req.GetName()))
	})
}

type testResources struct {
	accounts           []*identitycenterv1.Account
	permissionSets     []*identitycenterv1.PermissionSet
	assignments        []assignment
	accountAssignments map[assignment]*identitycenterv1.AccountAssignment
	roles              map[assignment]types.Role
}

type index struct {
	account int
	ps      int
}

func (tr *testResources) getAssignment(i index) assignment {
	return assignment{
		accountID:        tr.accounts[i.account].GetMetadata().GetName(),
		permissionSetARN: tr.permissionSets[i.ps].GetSpec().GetArn(),
	}
}

func (tr *testResources) getAssignments(indices ...index) []assignment {
	if indices == nil {
		return nil
	}
	dst := make([]assignment, len(indices))
	for i, src := range indices {
		dst[i] = tr.getAssignment(src)
	}
	return dst
}

func (tr *testResources) getRole(i index) types.Role {
	return tr.roles[tr.getAssignment(i)]
}

func (tr *testResources) getRoles(indices ...index) []types.Role {
	if indices == nil {
		return nil
	}
	dst := make([]types.Role, len(indices))
	for i, src := range indices {
		dst[i] = tr.getRole(src)
	}
	return dst
}

func (tr *testResources) getResourceIDs(indices ...index) []types.ResourceID {
	if indices == nil {
		return nil
	}
	dst := make([]types.ResourceID, len(indices))
	for i, src := range indices {
		dst[i] = tr.getResourceID(src)
	}
	return dst
}

func (tr *testResources) getResourceID(i index) types.ResourceID {
	assignment := tr.accountAssignments[tr.getAssignment(i)]
	return types.ResourceID{
		ClusterName: "test",
		Kind:        types.KindIdentityCenterAccountAssignment,
		Name:        assignment.GetMetadata().GetName(),
	}
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

	accounts := make([]*identitycenterv1.Account, 4)
	var assignments []assignment
	accountAssignments := make(map[assignment]*identitycenterv1.AccountAssignment)
	accountAssignmentRoles := make(map[assignment]types.Role)
	for i := range accounts {
		accountID := strings.Repeat(strconv.Itoa(i), 8)
		account := ictest.Account{
			ID:             services.IdentityCenterAccountID(accountID),
			Name:           fmt.Sprintf("Account #%02d", i),
			ARN:            fmt.Sprintf("arn:aws:iam::%s:account/Account%02d", accountID, i),
			PermissionSets: slices.Values(permissionSets),
		}.Build()
		ceated, err := fixture.Auth.CreateIdentityCenterAccount2(ctx, account)
		require.NoError(t, err)
		accounts[i] = ceated

		for _, ps := range permissionSets {
			key := assignment{accountID: accountID, permissionSetARN: ps.Spec.Arn}
			assignments = append(assignments, key)

			assignment := ictest.AccountAssignment{
				ID:                fmt.Sprintf("%s--%s", accountID, ps.Metadata.Name),
				DisplayName:       fmt.Sprintf("%s on %s", ps.Spec.Name, account.Spec.Name),
				AccountID:         services.IdentityCenterAccountID(accountID),
				PermissionSetName: ps.Spec.Name,
				PermissionSetARN:  ps.Spec.Arn,
			}.Build()
			created, err := fixture.Auth.CreateIdentityCenterAccountAssignment(ctx, assignment)
			require.NoError(t, err)
			accountAssignments[key] = created

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
			ictest.AssertSequenceLength(c, len(accountAssignments), iciter.AllAccountAssignments(ctx, fixture.Auth.Cache))
		}
		require.EventuallyWithT(t, cachePopulated, time.Second, 10*time.Millisecond)
	}

	return testResources{
		accounts:           accounts,
		permissionSets:     permissionSets,
		assignments:        assignments,
		accountAssignments: accountAssignments,
		roles:              accountAssignmentRoles,
	}
}
