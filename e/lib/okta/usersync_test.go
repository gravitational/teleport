package okta

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
)

// getResultAs extracts a value from a testify mock argument collection and
// casts it to the desired type. Safely handles untyped `nil` result values
// by returning the zero value for type T.
//
// Any attempt to cast an argument value to an incompatible type will still
// panic.
func getResultAs[T any](result mock.Arguments, index int) T {
	untypedValue := result.Get(index)
	if untypedValue == nil {
		var zero T
		return zero
	}

	typedValue, ok := untypedValue.(T)
	if !ok {
		panic(fmt.Sprintf("getResultAs[%T](%d) failed because object %v wasn't correct type", typedValue, index, untypedValue))
	}

	return typedValue
}

type mockReconcilerAP struct {
	mock.Mock
}

func (m *mockReconcilerAP) GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error) {
	result := m.Called(ctx, user, withSecrets)
	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockReconcilerAP) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	result := m.Called(ctx, withSecrets)
	return getResultAs[[]types.User](result, 0), result.Error(1)
}

func (m *mockReconcilerAP) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	result := m.Called(ctx, user)
	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockReconcilerAP) DeleteUser(ctx context.Context, user string) error {
	result := m.Called(ctx, user)
	return result.Error(0)
}

func (m *mockReconcilerAP) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	result := m.Called(ctx, user)
	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockReconcilerAP) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	result := m.Called(ctx, inForceOnly, targets)
	return getResultAs[[]types.Lock](result, 0), result.Error(1)
}

func (m *mockReconcilerAP) UpsertLock(ctx context.Context, lock types.Lock) error {
	result := m.Called(ctx, lock)
	fn, isDelegate := result.Get(0).(func(context.Context, types.Lock) error)
	if isDelegate {
		return fn(ctx, lock)
	}
	return result.Error(0)
}

func (m *mockReconcilerAP) DeleteLock(ctx context.Context, name string) error {
	result := m.Called(ctx, name)
	return result.Error(0)
}

type mockEventEmitter struct {
	mock.Mock
}

func (m *mockEventEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	result := m.Called(ctx, event)
	return result.Error(0)
}

// Compile-time assertion that our mock meets the interface definition
var _ ReconcilerAccessPoint = (*mockReconcilerAP)(nil)

// someContext is an argument matcher for testify mocks that matches any context.
var someContext interface{} = mock.MatchedBy(func(context.Context) bool { return true })

var userSyncEvent interface{} = mock.MatchedBy(
	func(e apievents.AuditEvent) bool {
		return e.GetType() == events.OktaUserSyncEvent
	})

func mkUser(t *testing.T, name string) types.User {
	u, err := types.NewUser(name)
	require.NoError(t, err)
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
	meta.Labels[eteleport.OktaUserStatusLabel] = userStatusActive
	u.SetMetadata(meta)
	return u
}

func setUserOktaStatus(u types.User, status string) {
	labels := u.GetStaticLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[eteleport.OktaUserStatusLabel] = status
	u.SetStaticLabels(labels)
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

type userReconcilerFixture struct {
	accessPoint  *mockReconcilerAP
	eventEmitter *mockEventEmitter
}

func (f *userReconcilerFixture) AssertExpectations(t *testing.T) {
	f.accessPoint.AssertExpectations(t)
	f.eventEmitter.AssertExpectations(t)
}

func newTestReconciler(t *testing.T) (*userReconciler, *userReconcilerFixture) {
	fixture := &userReconcilerFixture{
		accessPoint:  &mockReconcilerAP{},
		eventEmitter: &mockEventEmitter{},
	}

	reconciler, err := newUserReconciler(userReconcilerConfig{
		clusterName: t.Name(),
		teleportAP:  fixture.accessPoint,
		emitter:     fixture.eventEmitter,
		userOrgURL:  testOrgURL,
		clock:       clockwork.NewFakeClock(),
		logger:      slog.With("test", t.Name()),
	})
	require.NoError(t, err)

	return reconciler, fixture
}

func setStaticLabel(u types.User, key, value string) {
	labels := u.GetStaticLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	u.SetStaticLabels(labels)
}

func TestReconcileUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("new user is created", func(t *testing.T) {
		// Given a Teleport cluster with at least one user...
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		teleportUsers := map[string]types.User{"shaggy": shaggy}

		// AND an upstream Okta org with a superset of the Teleport users
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		oktaUsers := map[string]types.User{
			"scooby": scooby,
			"shaggy": shaggy,
		}

		// And a Teleport access point rigged to expect a request to create the
		// `scooby` user
		reconcilerUnderTest, fixture := newTestReconciler(t)
		fixture.accessPoint.On("CreateUser", someContext, scooby).
			Return(scooby, nil)

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, created: 1, total: 2}.validate).
			Return(nil)

		fixture.accessPoint.On("GetLocks", someContext, true, mock.Anything).
			Return([]types.Lock{}, nil).
			Once()

		// WHEN I reconcile the users...
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// EXPECT that the operation succeeds
		require.NoError(t, err)

		// ALSO EXPECT that the creation is reflected in the collected stats
		require.Equal(t, 1, stats.created)
		require.Zero(t, stats.deleted)
		require.Zero(t, stats.modified)
		require.Equal(t, 2, stats.total())

		// ALSO EXPECT and that our mock "CreateUser" method was hit with the correct user
		fixture.AssertExpectations(t)
	})

	t.Run("duplicate username is not an error", func(t *testing.T) {
		// GIVEN a Teleport cluster with an arbitrary set of users...
		teleportUsers := map[string]types.User{}

		// ALSO GIVEN an Okta organization with a user that shares the username
		// of the non-okta user
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		oktaUsers := map[string]types.User{
			"shaggy": shaggy,
			"scooby": scooby,
		}

		// ALSO GIVEN a Teleport access point rigged to expect a request to
		// create the user "scooby", and reject it with "already exists"
		reconcilerUnderTest, fixture := newTestReconciler(t)
		fixture.accessPoint.On("CreateUser", someContext, scooby).
			Return(nil, trace.AlreadyExists("we've already got one"))
		// ...and also rigged to expect a request to create user "velma" and
		// report success
		fixture.accessPoint.On("CreateUser", someContext, shaggy).
			Return(shaggy, nil)

		fixture.accessPoint.On("GetLocks", someContext, true, mock.Anything).
			Return([]types.Lock{}, nil).
			Once()

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, created: 1, total: 1}.validate).
			Return(nil)

		// WHEN I reconcile the users...
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// EXPECT that the operation succeeds and that our mock "CreateUser"
		// methods were hit with the correct users. Put together, this means
		// that the AlreadyExists error was handled internally by `reconcileUsers`.
		require.NoError(t, err)
		fixture.AssertExpectations(t)

		// ALSO EXPECT that the (single) creation is reflected in the collected
		// stats
		require.Equal(t, 1, stats.created)
		require.Zero(t, stats.deleted)
		require.Zero(t, stats.modified)
		require.Equal(t, 1, stats.total())
	})

	t.Run("modified user is updated", func(t *testing.T) {
		// Given an Okta organization and Teleport cluster with the same users
		// but one has been changed in Okta...
		reconcilerUnderTest, fixture := newTestReconciler(t)

		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
		oktaScooby.SetTraits(map[string][]string{
			"okta/peers": {"fred", "daphne", "shaggy"},
		})

		teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
		teleportScooby.SetTraits(map[string][]string{
			"okta/peers": {"fred", "daphne", "shaggy", "scrappy"},
		})

		oktaUsers := map[string]types.User{
			"scooby": oktaScooby,
			"shaggy": shaggy,
		}

		teleportUsers := map[string]types.User{
			"shaggy": shaggy,
			"scooby": teleportScooby,
		}

		// ...and a Teleport access point rigged to expect a request to update the
		// user "scooby"...
		fixture.accessPoint.On("UpdateUser", someContext, oktaScooby).
			Return(oktaScooby, nil).
			Once()

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, modified: 1, total: 2}.validate).
			Return(nil)

		// WHEN I try to reconcile the users...
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// EXPECT that the operation succeeds and our mock "UpdateUser" and
		// "GetLocks" methods were hit with the correct user
		require.NoError(t, err)
		fixture.AssertExpectations(t)

		// ALSO EXPECT that the update is recorded in the returned stats values
		require.Zero(t, stats.created)
		require.Zero(t, stats.deleted)
		require.Equal(t, 1, stats.modified)
		require.Equal(t, 2, stats.total())
	})

	t.Run("obsolete user is deleted", func(t *testing.T) {
		// Given an Okta organization and a Teleport cluster, where
		// Teleport has one more user than Okta...
		reconcilerUnderTest, fixture := newTestReconciler(t)
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		scrappy := mkOktaUser(t, "scrappy", "SCRAPPY")

		oktaUsers := map[string]types.User{
			"scooby": scooby,
			"shaggy": shaggy,
		}

		teleportUsers := map[string]types.User{
			"shaggy":  shaggy,
			"scooby":  scooby,
			"scrappy": scrappy,
		}

		// ... and a users service that expects to delete the given user
		// and any associated locks
		fixture.accessPoint.On("DeleteUser", someContext, scrappy.GetName()).
			Return(nil).
			Once()
		fixture.accessPoint.On("UpsertLock", someContext, mock.Anything).
			Run(validateDeletionLock(t, 1, scrappy.GetName(), reconcilerUnderTest.cfg.clock)).
			Return(nil)

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, deleted: 1, total: 2}.validate).
			Return(nil)

		// When I attempt to reconcile the users
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mocked delete operation
		// was hit with the correct user.
		require.NoError(t, err)
		fixture.AssertExpectations(t)

		// ALSO EXPECT that the delete is recorded in the returned stats values
		require.Zero(t, stats.created)
		require.Equal(t, 1, stats.deleted)
		require.Zero(t, stats.modified)
		require.Equal(t, 2, stats.total())
	})

	t.Run("changed logins are deleted and recreated", func(t *testing.T) {
		// Given an Teleport cluster and an Okta organization where one of the
		// Okta users has changed their username....
		reconcilerUnderTest, fixture := newTestReconciler(t)
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		oldScooby := mkOktaUser(t, "scooby", "SCOOBY")
		newScooby := mkOktaUser(t, "5kөөß¥", "SCOOBY")
		scrappy := mkOktaUser(t, "scrappy", "SCRAPPY")

		oktaUsers := map[string]types.User{
			shaggy.GetName():    shaggy,
			newScooby.GetName(): newScooby,
			scrappy.GetName():   scrappy,
		}

		teleportUsers := map[string]types.User{
			shaggy.GetName():    shaggy,
			oldScooby.GetName(): oldScooby,
			scrappy.GetName():   scrappy,
		}

		// ... and a Teleport Access Point that has been rigged to expect
		//  * a request to delete the old user
		//  * a request to lock the old user, and
		//  * a request to create the new user
		fixture.accessPoint.On("DeleteUser", someContext, oldScooby.GetName()).
			Return(nil).
			Once()
		fixture.accessPoint.On("UpsertLock", someContext, mock.Anything).
			Run(validateDeletionLock(t, 1, oldScooby.GetName(), reconcilerUnderTest.cfg.clock)).
			Return(nil)
		fixture.accessPoint.On("CreateUser", someContext, newScooby).
			Return(newScooby, nil).
			Once()
		fixture.accessPoint.On("GetLocks", someContext, true, mock.Anything).
			Return([]types.Lock{}, nil).
			Once()
		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, created: 1, deleted: 1, total: 3}.validate).
			Return(nil)

		// When I attempt to reconcile the users...
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds and our mocked operations
		// were hit with the correct users.
		require.NoError(t, err)
		fixture.AssertExpectations(t)

		// ALSO EXPECT that the delete+create pair is recorded in the returned
		// stats values
		require.Equal(t, 1, stats.created)
		require.Equal(t, 1, stats.deleted)
		require.Zero(t, stats.modified)
		require.Equal(t, 3, stats.total())
	})

	t.Run("unchanged users are left alone", func(t *testing.T) {
		// Given an Okta organization with the same users...
		reconcilerUnderTest, fixture := newTestReconciler(t)
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		velma := mkOktaUser(t, "velma", "VELMA")

		oktaUsers := map[string]types.User{
			scooby.GetName(): scooby,
			shaggy.GetName(): shaggy,
			velma.GetName():  velma,
		}

		teleportUsers := map[string]types.User{
			shaggy.GetName(): shaggy,
			scooby.GetName(): scooby,
			velma.GetName():  velma,
		}

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, total: 3}.validate).
			Return(nil)

		// WHEN I attempt to reconcile the users
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// EXPECT that the operation succeeds and no methods are called on the
		// access point
		require.NoError(t, err)

		// ALSO EXPECT that the no changes are recorded in the returned stats
		// values
		require.Zero(t, stats.created)
		require.Zero(t, stats.deleted)
		require.Zero(t, stats.modified)
		require.Equal(t, 3, stats.total())
	})

	t.Run("unchanged users are left alone even with changes to ", func(t *testing.T) {
		// Given an Okta organization with the same users...
		reconcilerUnderTest, fixture := newTestReconciler(t)
		shaggyOkta := mkOktaUser(t, "shaggy", "SHAGGY")
		scoobyOkta := mkOktaUser(t, "scooby", "SCOOBY")
		velmaOkta := mkOktaUser(t, "velma", "VELMA")

		oktaUsers := map[string]types.User{
			scoobyOkta.GetName(): scoobyOkta,
			shaggyOkta.GetName(): shaggyOkta,
			velmaOkta.GetName():  velmaOkta,
		}

		// same users as before, but with different weak mfa devices
		shaggyTeleport := mkOktaUser(t, "shaggy", "SHAGGY")
		shaggyTeleport.SetWeakestDevice(types.MFADeviceKind_MFA_DEVICE_KIND_WEBAUTHN)
		scoobyTeleport := mkOktaUser(t, "scooby", "SCOOBY")
		scoobyTeleport.SetWeakestDevice(types.MFADeviceKind_MFA_DEVICE_KIND_TOTP)
		velmaTeleport := mkOktaUser(t, "velma", "VELMA")
		velmaTeleport.SetWeakestDevice(types.MFADeviceKind_MFA_DEVICE_KIND_UNSET)
		teleportUsers := map[string]types.User{
			shaggyTeleport.GetName(): shaggyTeleport,
			scoobyTeleport.GetName(): scoobyTeleport,
			velmaTeleport.GetName():  velmaTeleport,
		}

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, total: 3}.validate).
			Return(nil)

		// WHEN I attempt to reconcile the users
		stats, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// EXPECT that the operation succeeds and no methods are called on the
		// access point
		require.NoError(t, err)

		// ALSO EXPECT that the no changes are recorded in the returned stats
		// values
		require.Zero(t, stats.created)
		require.Zero(t, stats.deleted)
		require.Zero(t, stats.modified)
		require.Equal(t, 3, stats.total())
	})

	t.Run("lockable users are locked", func(t *testing.T) {
		for userStatus := range lockableStatuses {
			t.Run(userStatus, func(t *testing.T) {
				// Given a Teleport cluster with a user...
				teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
				setStaticLabel(teleportScooby, eteleport.OktaUserStatusLabel, userStatusActive)
				teleportUsers := map[string]types.User{"scooby": teleportScooby}

				// AND an upstream Okta organization with the same user, but
				// with the user of interest deactivated
				oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
				setStaticLabel(oktaScooby, eteleport.OktaUserStatusLabel, userStatus)
				oktaUsers := map[string]types.User{"scooby": oktaScooby}

				// AND a user service that expects to update and lock the user
				// of interest and ONLY the user of interest
				reconcilerUnderTest, fixture := newTestReconciler(t)
				fixture.accessPoint.On("UpdateUser", someContext, oktaScooby).
					Return(oktaScooby, nil).
					Once()

				fixture.accessPoint.On("UpsertLock", someContext, mock.Anything).
					Run(validateSuspensionLock(t, 1, oktaScooby.GetName(), reconcilerUnderTest.cfg.clock)).
					Return(nil)

				// And we expect a sync event wll be emitted
				fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
					Return(nil)

				// when I try to reconcile the users...
				_, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

				// expect that the operation succeeds and our all of our mock
				// methods have been hit with the correct user
				require.NoError(t, err)
				fixture.AssertExpectations(t)
			})
		}
	})

	t.Run("user locks are not recreated on every update", func(t *testing.T) {
		// Given a cluster with one suspended user...
		teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
		setStaticLabel(teleportScooby, eteleport.OktaUserStatusLabel, userStatusSuspended)
		teleportUsers := map[string]types.User{"scooby": teleportScooby}

		// and an Okta organization with the same user, who has been updated but
		// IS STILL SUSPENDED
		oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
		setStaticLabel(oktaScooby, eteleport.OktaUserStatusLabel, userStatusSuspended)
		oktaScooby.SetTraits(map[string][]string{
			"okta/peers": {"fred", "daphne", "shaggy"},
		})
		oktaUsers := map[string]types.User{"scooby": oktaScooby}

		// AND a user service that expects to update and lock the user
		// of interest and ONLY the user of interest
		reconcilerUnderTest, fixture := newTestReconciler(t)
		fixture.accessPoint.On("UpdateUser", someContext, oktaScooby).
			Return(oktaScooby, nil).
			Once()

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Return(nil)

		// When I try to reconcile the users, expect that the mock AccessPoint
		// won't panic due to unexpected calls to lock manipulation methods...
		_, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Also expect that the operation succeeds, and our our expected mock
		// has been hit with the correct user.
		require.NoError(t, err)
		fixture.AssertExpectations(t)
	})

	t.Run("unlockable users are unlocked", func(t *testing.T) {
		// users in these states should NOT have their accounts locked.
		// See https://help.okta.com/en-us/content/topics/users-groups-profiles/usgp-end-user-states.htm
		// for the list of applicable user state values and what they mean
		validUserStates := []string{
			userStatusStaged, userStatusProvisioned, userStatusActive,
			userStatusRecovery, userStatusPasswordExpired,
		}

		for _, userStatus := range validUserStates {
			t.Run(userStatus, func(t *testing.T) {
				// Given an Okta organization and Teleport cluster with the
				// same user, but that user has been deactivated in Teleport and
				// re-activated in Okta.
				reconcilerUnderTest, fixture := newTestReconciler(t)

				teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
				setStaticLabel(teleportScooby, eteleport.OktaUserStatusLabel, "SUSPENDED")

				oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
				setStaticLabel(oktaScooby, eteleport.OktaUserStatusLabel, userStatus)

				oktaUsers := map[string]types.User{
					"scooby": oktaScooby,
				}

				teleportUsers := map[string]types.User{
					"scooby": teleportScooby,
				}

				scoobyLock := mkTestLockFor(teleportScooby)

				// Also given a teleport AccessPoint that rigged to expect an
				// update on user "scooby"
				fixture.accessPoint.On("UpdateUser", someContext, oktaScooby).
					Return(oktaScooby, nil).
					Once()

				fixture.accessPoint.On("GetLocks", someContext, true, mkLockTargetsFor(oktaScooby)).
					Return([]types.Lock{scoobyLock}, nil).
					Once()

				fixture.accessPoint.On("DeleteLock", someContext, scoobyLock.GetName()).
					Return(nil)

				// And we expect a sync event wll be emitted
				fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
					Return(nil)

				// when I try to reconcile the users...
				_, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

				// Expect that the operation succeeds and our mock "UpdateUser"
				// method was hit with the correct user
				require.NoError(t, err)
				fixture.AssertExpectations(t)
			})
		}
	})

	t.Run("non-okta locks are not deleted", func(t *testing.T) {
		// Given a Teleport cluster with a locked user, and that user has
		// multiple locks
		teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
		setStaticLabel(teleportScooby, eteleport.OktaUserStatusLabel, userStatusSuspended)
		teleportUsers := map[string]types.User{"scooby": teleportScooby}
		teleportLocks := []types.Lock{
			mkNonOktaTestLockFor(teleportScooby),
			mkTestLockFor(teleportScooby),
			mkNonOktaTestLockFor(teleportScooby),
			mkTestLockFor(teleportScooby),
			mkNonOktaTestLockFor(teleportScooby),
			mkNonOktaTestLockFor(teleportScooby),
			mkTestLockFor(teleportScooby),
			mkNonOktaTestLockFor(teleportScooby),
			mkTestLockFor(teleportScooby),
		}

		// and an Okta organization where that same user has been reactivated
		oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
		oktaUsers := map[string]types.User{"scooby": oktaScooby}

		// Also given a teleport AccessPoint that rigged to expect an
		// update on user "scooby" and return the above user set of locks when
		// asked
		reconcilerUnderTest, fixture := newTestReconciler(t)
		fixture.accessPoint.On("UpdateUser", someContext, oktaScooby).
			Return(oktaScooby, nil).
			Once()
		fixture.accessPoint.On("GetLocks", someContext, true, mkLockTargetsFor(oktaScooby)).
			Return(teleportLocks, nil)

		// ... and also rigged to expect calls to delete ONLY the Okta locks
		for _, l := range teleportLocks {
			if l.Origin() != types.OriginOkta {
				continue
			}

			fixture.accessPoint.On("DeleteLock", someContext, l.GetName()).
				Return(nil).
				Once()
		}

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Return(nil)

		// When I reconcile the users, expect that the mock access point doesn't
		// panic due to unexpected calls to "DeleteLock"
		_, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation succeeds
		require.NoError(t, err)

		// also, finally, expect that all of our expected mock methods were hit.
		// All taken together this implies that the non-Okta locks were left
		// alone while the Okta locks were deleted
		fixture.AssertExpectations(t)
	})

	t.Run("failing lock deletion does not prevent other lock deletions", func(t *testing.T) {
		// Given a Teleport cluster with a locked user, and that user has
		// multiple locks
		teleportScooby := mkOktaUser(t, "scooby", "SCOOBY")
		setStaticLabel(teleportScooby, eteleport.OktaUserStatusLabel, "SUSPENDED")
		teleportUsers := map[string]types.User{"scooby": teleportScooby}
		teleportLocks := []types.Lock{
			mkTestLockFor(teleportScooby),
			mkTestLockFor(teleportScooby),
			mkTestLockFor(teleportScooby),
		}

		// Also given an Okta organization where the user of interest has been
		// re-activated
		oktaScooby := mkOktaUser(t, "scooby", "SCOOBY")
		setStaticLabel(oktaScooby, eteleport.OktaUserStatusLabel, userStatusActive)
		oktaUsers := map[string]types.User{"scooby": oktaScooby}

		// Also given a teleport AccessPoint that rigged to expect an update on
		// user "scooby"
		reconcilerUnderTest, fixture := newTestReconciler(t)
		fixture.accessPoint.On("UpdateUser", someContext, oktaScooby).
			Return(oktaScooby, nil).
			Once()

		// ... and to list the locks on user "scooby", returning the lock list
		// created above
		fixture.accessPoint.On("GetLocks", someContext, true, mkLockTargetsFor(oktaScooby)).
			Return(teleportLocks, nil).
			Once()

		// ... and to delete locks, except that we fail for some
		// reason on the middle lock.
		fixture.accessPoint.On("DeleteLock", someContext, teleportLocks[0].GetName()).
			Return(nil)
		fixture.accessPoint.On("DeleteLock", someContext, teleportLocks[1].GetName()).
			Return(trace.AccessDenied("nope. not yours."))
		fixture.accessPoint.On("DeleteLock", someContext, teleportLocks[2].GetName()).
			Return(nil)

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Return(nil)

		// when I try to reconcile the users...
		_, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, teleportUsers)

		// Expect that the operation fails due to the lock deletion failure
		// method was hit with the correct user
		require.Error(t, err)

		// ... and that ALL of our expected method have been hit, implying that
		// the locks in slots #0 and #2 were deleted,m despite the middle one
		// failing.
		fixture.AssertExpectations(t)
	})

	t.Run("user lock is removed when user is created", func(t *testing.T) {
		scooby := mkOktaUser(t, "scooby", "SCOOBY")
		shaggy := mkOktaUser(t, "shaggy", "SHAGGY")

		setUserOktaStatus(shaggy, userStatusDeprovisioned)

		oktaUsers := map[string]types.User{
			"scooby": scooby,
			"shaggy": shaggy,
		}
		teleportLocks := []types.Lock{
			mkTestLockFor(scooby),
		}

		// And a Teleport access point rigged to expect a request to create the
		// `scooby` user
		reconcilerUnderTest, fixture := newTestReconciler(t)
		fixture.accessPoint.On("CreateUser", someContext, mock.Anything).
			Return(scooby, nil)

		// And we expect a sync event wll be emitted
		fixture.eventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
			Run(expectedUserSyncEvent{t: t, created: 2, total: 2}.validate).
			Return(nil)

		fixture.accessPoint.On("GetLocks", someContext, true, mock.Anything).
			Return(teleportLocks, nil).
			Once()

		fixture.accessPoint.On("DeleteLock", someContext, teleportLocks[0].GetName()).
			Return(nil)

		// When I reconcile the users...
		_, err := reconcilerUnderTest.reconcileUsers(ctx, oktaUsers, map[string]types.User{})

		// Expect that the operation succeeds
		require.NoError(t, err)

		// and that our mock "CreateUser" method was hit with the correct user
		fixture.AssertExpectations(t)
	})
}

// expectedUserSyncEvent describes the expected values in a user sync event.
// Designed to be a succinct way of stating the expected stats values carried by
// the event without making them be an opaque sequence of numbers at the call
// site.
//
// Example usage:
//
//	  mockEventEmitter.On("EmitAuditEvent", someContext, userSyncEvent).
//			Run(expectedUserSyncEvent{t: t, created: 1, total: 3}.validate).
//			Return(nil)
type expectedUserSyncEvent struct {
	t        *testing.T
	created  int32
	modified int32
	deleted  int32
	total    int32
}

// validate asserts the correctness of the stats in an OktaUserSync event.
// Expected to be used as a testify Mock `Run()` callback
func (expected expectedUserSyncEvent) validate(args mock.Arguments) {
	syncEvent, ok := getResultAs[apievents.AuditEvent](args, 1).(*apievents.OktaUserSync)
	require.True(expected.t, ok, "Expecting OktaUserSync event")

	require.Equal(expected.t, expected.t.Name(), syncEvent.GetClusterName())
	require.Equal(expected.t, events.OktaUserSyncSuccessCode, syncEvent.Code)
	require.Equal(expected.t, events.OktaUserSyncEvent, syncEvent.GetType())
	require.Equal(expected.t, testOrgURL, syncEvent.OrgUrl)
	require.True(expected.t, syncEvent.Status.Success)

	require.Equal(expected.t, expected.created, syncEvent.NumUsersCreated, "Created")
	require.Equal(expected.t, expected.modified, syncEvent.NumUsersModified, "Modified")
	require.Equal(expected.t, expected.deleted, syncEvent.NumUsersDeleted, "Deleted")
	require.Equal(expected.t, expected.total, syncEvent.NumUsersTotal, "Total")
}

func validateDeletionLock(t *testing.T, argIndex int, target string, clock clockwork.Clock) func(mock.Arguments) {
	expiry := clock.Now().Add(lockTTL)
	return validateLockArg(t, argIndex, target, expiry, LockReasonDeleted)
}

func validateSuspensionLock(t *testing.T, argIndex int, target string, clock clockwork.Clock) func(mock.Arguments) {
	expiry := clock.Now().Add(lockTTL)
	return validateLockArg(t, argIndex, target, expiry, LockReasonSuspended)
}

func validateLockArg(t *testing.T, argIndex int, target string, _ time.Time, reason string) func(mock.Arguments) {
	return func(args mock.Arguments) {
		lock := args.Get(argIndex).(types.Lock)
		require.NotEmpty(t, lock.GetName())

		// Expect that the origin label is set
		require.Equal(t, types.OriginOkta, lock.Origin())

		// Expect that the Upstream Org URL is set
		orgUrl, ok := lock.GetLabel(eteleport.OktaOrgURLLabel)
		require.True(t, ok)
		require.Equal(t, testOrgURL, orgUrl)

		// Expect that the reason label is set
		lockReason, ok := lock.GetLabel(eteleport.OktaLockReasonLabel)
		require.True(t, ok)
		require.Equal(t, reason, lockReason)

		require.Nil(t, lock.LockExpiry())

		// Expect that the target is set to the supplied username
		// *and nothing else*
		require.Equal(t, types.LockTarget{User: target}, lock.Target())
	}
}

// mkTestLockFor creates a new lock targeting the supplied user. The lock name
// is random so it is safe to create multiple locks on the same user in the same
// test.
func mkNonOktaTestLockFor(user types.User) types.Lock {
	l := &types.LockV2{
		Metadata: types.Metadata{
			Name:   uuid.NewString(),
			Labels: map[string]string{},
		},
		Spec: types.LockSpecV2{
			Message: "No Access For You",
			Target: types.LockTarget{
				User: user.GetName(),
			},
			CreatedBy: types.SystemResource,
		},
	}
	if err := l.CheckAndSetDefaults(); err != nil {
		panic(err)
	}
	return l
}

// mkTestLockFor creates a new suspension lock targeting the supplied user. The
// lock name is random so it is safe to create multiple locks on the same user
// in the same test.
func mkTestLockFor(user types.User) types.Lock {
	l := &types.LockV2{
		Metadata: types.Metadata{
			Name: uuid.NewString(),
			Labels: map[string]string{
				types.OriginLabel:             types.OriginOkta,
				eteleport.OktaOrgURLLabel:     testOrgURL,
				eteleport.OktaLockReasonLabel: LockReasonSuspended,
			},
		},
		Spec: types.LockSpecV2{
			Message: "No Access For You",
			Target: types.LockTarget{
				User: user.GetName(),
			},
			CreatedBy: types.OriginOkta,
		},
	}
	if err := l.CheckAndSetDefaults(); err != nil {
		panic(err)
	}
	return l
}

func mkLockTargetsFor(users ...types.User) []types.LockTarget {
	result := make([]types.LockTarget, len(users))
	for i, user := range users {
		result[i] = types.LockTarget{User: user.GetName()}
	}
	return result
}

func TestLockUser(t *testing.T) {
	mockClock := clockwork.NewFakeClock()
	mockAccessPoint := &mockReconcilerAP{}
	targetUser := mkOktaUser(t, "hiro@enzos-pizza.com", "00ub1q9yfsRSfO91a5d7")

	targetedLock := mock.MatchedBy(func(l types.Lock) bool {
		return l.Target().User == targetUser.GetName()
	})

	mockAccessPoint.
		On("UpsertLock", someContext, targetedLock).
		Return(nil)

	lock, err := LockUser(context.Background(), LockParams{
		User:     targetUser,
		Reason:   LockReasonSuspended,
		OrgURL:   "https://enzos-pizza.okta.com",
		LocksSvc: mockAccessPoint,
		Clock:    mockClock,
	})

	require.NoError(t, err)
	require.NotNil(t, lock)
}

type mockIsLeader struct {
	isLeader bool
}

func (m *mockIsLeader) IsLeader() bool {
	return m.isLeader
}
