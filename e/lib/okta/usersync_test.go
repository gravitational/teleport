package okta

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

type mockReconcilerAP struct {
	mock.Mock
}

func (m *mockReconcilerAP) GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error) {
	result := m.Called(ctx, user, withSecrets)
	return result.Get(0).(types.User), result.Error(1)
}

func (m *mockReconcilerAP) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	result := m.Called(ctx, withSecrets)
	return result.Get(0).([]types.User), result.Error(1)
}

func (m *mockReconcilerAP) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	result := m.Called(ctx, user)
	return result.Get(0).(types.User), result.Error(1)
}

func (m *mockReconcilerAP) DeleteUser(ctx context.Context, user string) error {
	result := m.Called(ctx, user)
	return result.Error(0)
}

func (m *mockReconcilerAP) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	result := m.Called(ctx, user)
	maybeUser := result.Get(0)
	var userResult types.User
	if maybeUser != nil {
		userResult = maybeUser.(types.User)
	}
	return userResult, result.Error(1)
}

// Compile-time assertion that our mock meets the interface definition
var _ ReconcilerAccessPoint = (*mockReconcilerAP)(nil)

// someContext is an argument matcher for testify mocks that matches any context.
var someContext interface{} = mock.MatchedBy(func(context.Context) bool { return true })

func mkUser(t *testing.T, name string) types.User {
	u, err := types.NewUser(name)
	require.NoError(t, err)
	require.NoError(t, u.CheckAndSetDefaults())
	return u
}

func mkOktaUser(t *testing.T, name, oktaUserID string) types.User {
	u := mkUser(t, name)
	meta := u.GetMetadata()
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	meta.Labels[types.OriginLabel] = types.OriginOkta
	meta.Labels[eteleport.OktaOrgURLLabel] = testOrgURL
	meta.Labels[eteleport.OktaUserIDLabel] = oktaUserID
	u.SetMetadata(meta)
	return u
}

func TestListTeleportUsers(t *testing.T) {
	ctx := context.Background()
	log := logrus.WithField("test", t.Name())

	t.Run("Empty list is not an error", func(t *testing.T) {
		mockUsersSvc := &mockReconcilerAP{}
		mockUsersSvc.
			On("GetUsers", someContext, false).
			Return([]types.User{}, nil)

		users, err := listTeleportUsers(ctx, mockUsersSvc, testOrgURL, log)
		require.NoError(t, err)
		require.Empty(t, users)
	})

	t.Run("list filters to okta users", func(t *testing.T) {
		// Give a Teleport cluster with a mix of local users, and users imported
		// from Okta (including multiple organizations)

		// Scooby & Shaggy are imported from Okta
		scooby := mkOktaUser(t, "scooby", "0001")
		shaggy := mkOktaUser(t, "shaggy", "0002")

		// Fred is from a different Okta org, and uas a userID clash with scooby
		fred := mkOktaUser(t, "fred", "0001")
		fred.GetMetadata().Labels[eteleport.OktaOrgURLLabel] = "https://somewhere.else.com"

		// Daphne and Velma are just regular Teleport users
		daphne := mkUser(t, "daphne")
		velma := mkUser(t, "velma")

		mockUsersSvc := &mockReconcilerAP{}
		mockUsersSvc.
			On("GetUsers", someContext, false).
			Return([]types.User{scooby, shaggy, fred, daphne, velma}, nil)

		// When I list the teleport users requiring reconciliation
		users, err := listTeleportUsers(ctx, mockUsersSvc, testOrgURL, log)

		// Expect the returned list contains only those teleport users with the
		// correct Okta attributes set
		require.NoError(t, err)
		require.Len(t, users, 2)
		require.Contains(t, users, scooby.GetName())
		require.Contains(t, users, shaggy.GetName())
	})
}

func newTestReconciler(t *testing.T) (*userReconciler, *mockReconcilerAP) {
	accessPoint := &mockReconcilerAP{}
	reconciler, err := newUserReconciler(userReconcilerConfig{
		teleportAP: accessPoint,
		userOrgURL: "https://mystery-machine.okta.org",
		log:        logrus.WithField("test", t.Name()),
	})
	require.NoError(t, err)

	return reconciler, accessPoint
}

func TestReconcileUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("new user is created", func(t *testing.T) {
		// Given an Okta organization with a user not In the Teleport user
		// database...
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")

		uut, ap := newTestReconciler(t)
		ap.On("CreateUser", someContext, scooby).
			Return(scooby, nil)

		oktaUsers := types.ResourcesWithLabelsMap{
			"scooby": scooby,
			"shaggy": shaggy,
		}

		teleportUsers := types.ResourcesWithLabelsMap{
			"shaggy": shaggy,
		}

		// When I reconcile the users...
		err := uut.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mock "CreateUser"
		// method was hit with the correct user
		require.NoError(t, err)
		ap.AssertExpectations(t)
	})

	t.Run("duplicate username is not an error", func(t *testing.T) {
		// Given an Okta organization with a user not In the Teleport user
		// database...
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")

		uut, ap := newTestReconciler(t)
		ap.On("CreateUser", someContext, scooby).
			Return(nil, trace.AlreadyExists("duplicate username"))

		oktaUsers := types.ResourcesWithLabelsMap{
			"scooby": scooby,
			"shaggy": shaggy,
		}

		teleportUsers := types.ResourcesWithLabelsMap{
			"shaggy": shaggy,
		}

		// When I reconcile the users...
		err := uut.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mock "CreateUser" method
		// was hit with the correct user. Put together, this means that the
		// AlreadyExists error was handled internally by `reconcileUsers`.
		require.NoError(t, err)
		ap.AssertExpectations(t)
	})

	t.Run("modified user is updated", func(t *testing.T) {
		// Given an Okta organization and Teleport cluster with the same users
		// but one has been changed in Okta...
		uut, ap := newTestReconciler(t)

		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
		oktaScooby.SetTraits(map[string][]string{
			"okta/peers": {"fred", "daphne", "shaggy"},
		})

		teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
		teleportScooby.SetTraits(map[string][]string{
			"okta/peers": {"fred", "daphne", "shaggy", "scrappy"},
		})

		oktaUsers := types.ResourcesWithLabelsMap{
			"scooby": oktaScooby,
			"shaggy": shaggy,
		}

		teleportUsers := types.ResourcesWithLabelsMap{
			"shaggy": shaggy,
			"scooby": teleportScooby,
		}

		// ... and a user service that expects to only update the given user
		ap.On("UpdateUser", someContext, oktaScooby).
			Return(oktaScooby, nil).
			Once()

		// when I try to reconcile the users...
		err := uut.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mock "UpdateUser"
		// method was hit with the correct user
		require.NoError(t, err)
		ap.AssertExpectations(t)
	})

	t.Run("obsolete user is deleted", func(t *testing.T) {
		// Given an Okta organization and a Teleport cluster, where Teleport has
		// one more user than Okta...
		uut, ap := newTestReconciler(t)
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		scrappy := mkOktaUser(t, "scrappy", "SCRAPPY")

		oktaUsers := types.ResourcesWithLabelsMap{
			"scooby": scooby,
			"shaggy": shaggy,
		}

		teleportUsers := types.ResourcesWithLabelsMap{
			"shaggy":  shaggy,
			"scooby":  scooby,
			"scrappy": scrappy,
		}

		// ... and a users service that expects to delete the given user
		ap.On("DeleteUser", someContext, scrappy.GetName()).
			Return(nil).
			Once()

		// When I attempt to reconcile the users
		err := uut.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mocked delete operation
		// was hit with the correct user.
		require.NoError(t, err)
		ap.AssertExpectations(t)
	})

	t.Run("changed logins are deleted and recreated", func(t *testing.T) {
		// Given an Teleport cluster and an Okta organization where one of the
		// Okta users has changed their username....
		uut, ap := newTestReconciler(t)
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		oldScooby := mkOktaUser(t, "scooby", "SCOOBY")
		newScooby := mkOktaUser(t, "5kөөß¥", "SCOOBY")
		scrappy := mkOktaUser(t, "scrappy", "SCRAPPY")

		oktaUsers := types.ResourcesWithLabelsMap{
			shaggy.GetName():    shaggy,
			newScooby.GetName(): newScooby,
			scrappy.GetName():   scrappy,
		}

		teleportUsers := types.ResourcesWithLabelsMap{
			shaggy.GetName():    shaggy,
			oldScooby.GetName(): oldScooby,
			scrappy.GetName():   scrappy,
		}

		// ... and some Teleport services that have been rigged to succeed
		ap.On("DeleteUser", someContext, oldScooby.GetName()).
			Return(nil).
			Once()
		ap.On("CreateUser", someContext, newScooby).
			Return(newScooby, nil).
			Once()

		// When I attempt to reconcile the users...
		err := uut.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mocked operations
		// were hit with the correct users.
		require.NoError(t, err)
		ap.AssertExpectations(t)
	})

	t.Run("unchanged users are left alone", func(t *testing.T) {
		// Given an Okta organization with the same users...
		uut, _ := newTestReconciler(t)
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		velma := mkOktaUser(t, "velma", "VELMA")

		oktaUsers := types.ResourcesWithLabelsMap{
			scooby.GetName(): scooby,
			shaggy.GetName(): shaggy,
			velma.GetName():  velma,
		}

		teleportUsers := types.ResourcesWithLabelsMap{
			shaggy.GetName(): shaggy,
			scooby.GetName(): scooby,
			velma.GetName():  velma,
		}

		// When I attempt to reconcile the users
		err := uut.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and no methods are called on the
		// access point
		require.NoError(t, err)
	})
}
