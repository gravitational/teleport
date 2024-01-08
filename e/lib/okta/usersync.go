package okta

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// lockTTL is the TTL of a lock created when a user is suspended or deleted.
	// These locks need to live long enough to prevent the deleted user from
	// creating new sessions while any credentials they have may be valid, plus
	// a buffer
	lockTTL = apidefaults.MaxCertDuration + (10 * time.Minute)

	// oktaLockReasonSuspended indicates that a lock's purpose is to lock out an
	// Okta user who is suspended by the upstream Okta organization. These locks
	// remain in force until they are explicitly deleted
	oktaLockReasonSuspended = "suspended"

	// oktaLockReasonDeleted indicates that a lock's purpose is to lock out an
	// Okta user who was deleted by the upstream Okta organization. These locks
	// only remain in force temporarily, while the user is being deleted.
	oktaLockReasonDeleted = "deleted"
)

// Status values drawn from https://github.com/okta/okta-sdk-java/blob/master/src/swagger/api.yaml
// Values interpreted as per https://help.okta.com/en-us/content/topics/users-groups-profiles/usgp-end-user-states.htm
const (
	userStatusStaged          = "STAGED"
	userStatusProvisioned     = "PROVISIONED"
	userStatusActive          = "ACTIVE"
	userStatusRecovery        = "RECOVERY"
	userStatusPasswordExpired = "PASSWORD_EXPIRED"
	userStatusLockedOut       = "LOCKED_OUT"
	userStatusSuspended       = "SUSPENDED"
	userStatusDeprovisioned   = "DEPROVISIONED"
)

// ReconcilerAccessPoint provides CRUD methods on the Teleport cluster's
// user database used by the Okta user reconciler.
type ReconcilerAccessPoint interface {
	// CreateUserWithContext creates a user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)

	// GetUserWithContext returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)

	// GetUsersWithContext returns a list of users registered with the local
	// cluster auth server.
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)

	// UpdateUser updates the backend record to match the supplied struct,
	// returning the updated user.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)

	// DeleteUser deletes the user with the given name
	DeleteUser(ctx context.Context, user string) error

	// GetLocks lists the locks that target a given set of resources.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)

	// UpsertLock creates or updates a given lock
	UpsertLock(ctx context.Context, lock types.Lock) error

	// DeleteLock deletes a given lock
	DeleteLock(ctx context.Context, name string) error
}

// userConverter is a function that can convert an Okta user into a Teleport
// user.
type userConverter func(*okta.User) (types.User, error)

// fetchOktaUsers fetches users from the upstream okta service and creates
// candidate Teleport user equivalents for them.
func fetchOktaUsers(ctx context.Context, oktaClient oktaClient, convertUser userConverter, log logrus.FieldLogger) (map[string]types.User, error) {
	result := map[string]types.User{}
	err := oktaClient.iterateUsers(ctx, func(ou *okta.User) error {
		log.Debugf("Processing Okta user %s...", ou.Id)
		teleportUser, err := convertUser(ou)
		if err != nil {
			log.WithError(err).Warnf("Failed converting user %s. Skipping.", ou.Id)
		}
		result[teleportUser.GetName()] = teleportUser
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err, "enumerating Okta users")
	}
	return result, nil
}

// listTeleportUsers lists the teleport users that are managed by the Okta
// integration. Users for individual integrations are differentiated by the
// `userOrgURL`.
func listTeleportUsers(ctx context.Context, userSvc ReconcilerAccessPoint, userOrgURL string, log logrus.FieldLogger) (map[string]types.User, error) {
	result := map[string]types.User{}

	users, err := userSvc.GetUsers(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err, "listing teleport okta users")
	}

	for _, user := range users {
		// Filter out non-okta-origin users
		if user.Origin() != types.OriginOkta {
			continue
		}

		// Filter out okta users from a different Okta organization, which
		// belong to a different integration
		if label, ok := user.GetLabel(eteleport.OktaOrgURLLabel); !ok || label != userOrgURL {
			continue
		}

		result[user.GetName()] = user
	}

	return result, nil
}

// userReconcilerConfig holds the caller-supplied information needed to create a
// userReconciler.
type userReconcilerConfig struct {
	teleportAP ReconcilerAccessPoint
	userOrgURL string
	clock      clockwork.Clock
	log        logrus.FieldLogger
}

// CheckAndSetDefaults validates the config, supplying defaults as necessary
func (cfg *userReconcilerConfig) CheckAndSetDefaults() error {
	if cfg.teleportAP == nil {
		return trace.BadParameter("missing access point")
	}

	if cfg.userOrgURL == "" {
		return trace.BadParameter("missing Okta org url")
	}

	if cfg.log == nil {
		cfg.log = logrus.WithField(trace.Component, eteleport.ComponentOkta)
	}

	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}

	return nil
}

// userReconciler reconciles Teleport users with an upstream Okta organization,
// creating, updating and deleting Teleport users as necessary.
type userReconciler struct {
	cfg           userReconcilerConfig
	backend       *services.Reconciler[types.User]
	teleportUsers map[string]types.User
	oktaUsers     map[string]types.User
}

// newUserReconciler constructs a new userReconciler from the supplied config.
// No actual reconciliation ids attempted until
func newUserReconciler(cfg userReconcilerConfig) (*userReconciler, error) {
	var err error

	if err = cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	userReconciler := &userReconciler{
		cfg: cfg,
	}

	userReconciler.backend, err = services.NewReconciler(
		services.ReconcilerConfig[types.User]{
			Matcher:             func(r types.User) bool { return true },
			GetCurrentResources: userReconciler.getTeleportUsers,
			GetNewResources:     userReconciler.getOktaUsers,
			OnCreate:            userReconciler.createTeleportUser,
			OnUpdate:            userReconciler.updateTeleportUser,
			OnDelete:            userReconciler.deleteTeleportUser,
			Log:                 cfg.log,
		})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userReconciler, nil
}

// getTeleportUsers supplies the "current" Teleport resources to the inner
// Reconciler
func (r *userReconciler) getTeleportUsers() map[string]types.User {
	return r.teleportUsers
}

// getTeleportUsers supplies the "new" Okta-derived user resources to the inner
// Reconciler
func (r *userReconciler) getOktaUsers() map[string]types.User {
	return r.oktaUsers
}

// createTeleportUser is the `OnCreate` delegate for the inner reconciler.
// Creates the Teleport user record for the supplied user.
func (r *userReconciler) createTeleportUser(ctx context.Context, oktaUser types.User) error {
	// This method does NOT create a Teleport lock if the supplied Okta user is
	// suspended, or in an otherwise lockable state. This is because the sync
	// service uses locks as a mechanism to immediately terminate active user
	// sessions as soon as we know the user has been disabled. No such sessions
	// can exist for a newly-created user, so no Teleport-side lock is necessary.
	//
	// As for preventing login, an Okta user *must* log in via SAML, and as such
	// the upstream Okta organization itself will prevent the disabled user from
	// gaining credentials to start any new sessions.

	oktaUserID, ok := oktaUser.GetLabel(eteleport.OktaUserIDLabel)
	if !ok {
		oktaUserID = "<UNKNOWN>"
	}

	if _, err := r.cfg.teleportAP.CreateUser(ctx, oktaUser); err != nil {
		// Trying to overwrite an existing user isn't allowed, but it is not a
		// condition we should fail the sync pass for.
		if trace.IsAlreadyExists(err) {
			r.cfg.log.
				WithFields(logrus.Fields{
					"user_name":    oktaUser.GetName(),
					"okta_user_id": oktaUserID,
				}).
				Warnf("User already exists in Teleport and was not imported.")
			return nil
		}

		return trace.Wrap(err)
	}

	return nil
}

// updateTeleportUser is the `OnUpdate` delegate for the inner reconciler
func (r *userReconciler) updateTeleportUser(ctx context.Context, newUser, oldUser types.User) error {
	if _, err := r.cfg.teleportAP.UpdateUser(ctx, newUser); err != nil {
		return trace.Wrap(err, "updating user %q", newUser.GetName())
	}

	oldUserLocked := userHasLockableStatus(oldUser)
	newUserLocked := userHasLockableStatus(newUser)
	transitionToLock := !oldUserLocked && newUserLocked
	transitionOutOfLock := oldUserLocked && !newUserLocked

	switch {
	case transitionToLock:
		// Create an okta lock on the user
		if _, err := r.lockUser(ctx, newUser, oktaLockReasonSuspended); err != nil {
			return trace.Wrap(err, "locking on user %q", newUser.GetName())
		}

	case transitionOutOfLock:
		locks, err := r.getOktaLocksForUser(ctx, newUser, oktaLockReasonSuspended)
		if err != nil {
			return trace.Wrap(err, "fetching locks on user %q", newUser.GetName())
		}

		var lockErrors []error
		for _, lock := range locks {
			err := r.cfg.teleportAP.DeleteLock(ctx, lock.GetName())
			if err != nil && !trace.IsNotFound(err) {
				lockErrors = append(lockErrors,
					trace.Wrap(err,
						"deleting lock %s on user %q", lock.GetName(), newUser.GetName()))
			}
		}

		return trace.NewAggregate(lockErrors...)
	}

	return nil
}

// deleteTeleportUser is the `OnDelete` delegate for the inner reconciler. It
// deletes the given user, but leaves any existing suspension locks on that user
// to eventually expire naturally (as opposed to explicitly deleting them along
// with the user). We don't want to open up a hole where a user may be able to
// use already-issued credentials.
func (r *userReconciler) deleteTeleportUser(ctx context.Context, user types.User) error {

	// Create a lock on the user so that any open sessions that they have are
	// terminated with extreme prejudice
	if _, err := r.lockUser(ctx, user, oktaLockReasonDeleted); err != nil {
		return trace.Wrap(err)
	}

	// Do the actual deletion
	if err := r.cfg.teleportAP.DeleteUser(ctx, user.GetName()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// reconcileUsers pulls the user list from an upstream okta organization and
// updates teleport users to match; creating, updating and deleting teleport
// users as needed. Reconciliation is strictly one-way; no attempts are made
// to update the upstream Okta organization.
func (r *userReconciler) reconcileUsers(ctx context.Context, oktaUsers, teleportUsers map[string]types.User) error {
	// Stash the supplied resource maps where the inner reconciler will be able
	// to find them
	r.oktaUsers = oktaUsers
	r.teleportUsers = teleportUsers
	defer func() {
		r.teleportUsers = nil
		r.oktaUsers = nil
	}()

	// There are several Teleport-generated fields that we want to ensure are
	// set exactly the same in the new candidate records from okta:
	//
	// * The user revision must be carried across verbatim, or Teleport's
	//   optimistic record locking will think that we based our changes off an
	//   old revision and reject the update.
	// * The creation info in the candidate record will have a timestamp of
	//   `now`, while the existing record will have an older timestamp which
	//   the reconciler will incorrectly interpret as a change
	for username, teleportUser := range teleportUsers {
		oktaUser, ok := oktaUsers[username]
		if !ok {
			continue
		}

		oktaUser.SetRevision(teleportUser.GetRevision())
		oktaUser.SetCreatedBy(teleportUser.GetCreatedBy())
	}

	// Run the reconciliation
	if err := r.backend.Reconcile(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// lockUser creates a Teleport lock targeting the supplied user. This will cause
// any open sessions held by that user to be immediately terminated.
func (r *userReconciler) lockUser(ctx context.Context, user types.User, reason string) (types.Lock, error) {
	expiry := r.cfg.clock.Now().Add(lockTTL)
	status, _ := user.GetLabel(eteleport.OktaUserStatusLabel)
	l := &types.LockV2{
		Metadata: types.Metadata{
			Name: uuid.NewString(),
			Labels: map[string]string{
				types.OriginLabel:             types.OriginOkta,
				eteleport.OktaOrgURLLabel:     r.cfg.userOrgURL,
				eteleport.OktaLockReasonLabel: reason,
			},
			Expires: &expiry,
		},
		Spec: types.LockSpecV2{
			Message: fmt.Sprintf("Okta user %q is %s", user.GetName(), status),
			Target: types.LockTarget{
				User: user.GetName(),
			},
			CreatedBy: types.OriginOkta,
			Expires:   &expiry,
		},
	}

	if err := l.CheckAndSetDefaults(); err != nil {
		r.cfg.log.WithError(err).Error("setting lock defaults")
		return nil, trace.Wrap(err, "setting lock defaults")
	}

	r.cfg.log.WithFields(
		logrus.Fields{
			"user_name": user.GetName(),
			"lock_name": l.GetName(),
		}).
		Debugf("Locking user %s", user.GetName())

	if err := r.cfg.teleportAP.UpsertLock(ctx, l); err != nil {
		return nil, trace.Wrap(err, "locking user %q", user.GetName())
	}

	return l, nil
}

// getOktaLocksForUser fetches all of the okta-created locks applied to the
// supplied user.
func (r *userReconciler) getOktaLocksForUser(ctx context.Context, user types.User, reason string) ([]types.Lock, error) {
	allLocks, err := r.cfg.teleportAP.GetLocks(ctx, true /* in force only */, types.LockTarget{User: user.GetName()})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var oktaLocks []types.Lock
	for _, lock := range allLocks {
		if isOktaLock(lock, r.cfg.userOrgURL, reason) {
			oktaLocks = append(oktaLocks, lock)
		}
	}

	return oktaLocks, nil
}

// lockableStatuses is the set of Okta user statuses from https://help.okta.com/en-us/content/topics/users-groups-profiles/usgp-end-user-states.htm
var lockableStatuses = map[string]struct{}{
	userStatusSuspended:     {},
	userStatusDeprovisioned: {},
	userStatusLockedOut:     {},
}

func userHasLockableStatus(user types.User) bool {
	status, _ := user.GetLabel(eteleport.OktaUserStatusLabel)
	_, isLockable := lockableStatuses[strings.ToUpper(status)]
	return isLockable
}

// isOktaLock identifies a Teleport lock as belonging to *this* Okta service by
// examining the lock origin and Okta services.
func isOktaLock(lock types.Lock, orgUrl string, reason string) bool {
	if lock.Origin() != types.OriginOkta {
		return false
	}

	if url, _ := lock.GetLabel(eteleport.OktaOrgURLLabel); url != orgUrl {
		return false
	}

	if lr, _ := lock.GetLabel(eteleport.OktaLockReasonLabel); lr != reason {
		return false
	}

	return true
}
