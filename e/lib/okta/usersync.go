package okta

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/okta/api"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// lockTTL is the TTL of a lock created when a user is suspended or deleted.
	// These locks need to live long enough to prevent the deleted user from
	// creating new sessions while any credentials they have may be valid, plus
	// a buffer
	lockTTL = apidefaults.MaxCertDuration + (10 * time.Minute)

	// LockReasonSuspended indicates that a lock's purpose is to lock out an
	// Okta user who is suspended by the upstream Okta organization. These locks
	// only remain in force temporarily until the maximum credential TTL has
	// expired, unless deleted earlier.
	LockReasonSuspended = "suspended"

	// LockReasonDeleted indicates that a lock's purpose is to lock out an
	// Okta user who was deleted by the upstream Okta organization. These locks
	// only remain in force temporarily, while the user is being deleted.
	LockReasonDeleted = "deleted"

	// LockReasonDeactivated indicates that a lock's purpose is to lock out an
	// Okta user who was deactivated  by the upstream Okta organization. These locks
	// only remain in force temporarily until the maximum credential TTL has
	// expired, unless deleted earlier
	LockReasonDeactivated = "deactivated"
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

// LocksService abstracts over the Teleport lock service, providing the subset
// of lock operations required by the UserReconciler.
type LocksService interface {
	// GetLocks lists the locks that target a given set of resources.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)

	// UpsertLock creates or updates a given lock
	UpsertLock(ctx context.Context, lock types.Lock) error

	// DeleteLock deletes a given lock
	DeleteLock(ctx context.Context, name string) error
}

// ReconcilerAccessPoint provides CRUD methods on the Teleport cluster's
// user database used by the Okta user reconciler.
type ReconcilerAccessPoint interface {
	LocksService

	// CreateUser creates a user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// GetUsers returns a list of users registered with the local
	// cluster auth server.
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)
	// UpdateUser updates the backend record to match the supplied struct,
	// returning the updated user.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
	// DeleteUser deletes the user with the given name.
	DeleteUser(ctx context.Context, user string) error
}

// userConverter is a function that can convert an Okta user into a Teleport
// user.
type userConverter func(*okta.User) (types.User, error)

// fetchOktaUsers fetches users from the upstream okta service and creates
// candidate Teleport user equivalents for them.
func fetchOktaUsers(ctx context.Context, oktaClient api.Client, convertUser userConverter, log *slog.Logger) (map[string]types.User, error) {
	result := map[string]types.User{}
	err := oktaClient.IterateUsers(ctx, func(ou *okta.User) error {
		log := log.With("okta_user_id", ou.Id)
		log.DebugContext(ctx, "Processing Okta user")

		if ou.Status == userStatusSuspended {
			log.DebugContext(ctx, "Skipping suspended user")
			return nil
		}

		teleportUser, err := convertUser(ou)
		if err != nil {
			log.WarnContext(ctx, "Skipping invalid user object", "error", err)
			return nil
		}
		result[teleportUser.GetName()] = teleportUser
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err, "enumerating Okta users")
	}
	return result, nil
}

// appUserConverter is a function that can convert an Okta app user into a
// Teleport user.
type appUserConverter func(*okta.AppUser) (types.User, error)

// fetchOktaUsers fetches users from the upstream okta service and creates
// candidate Teleport user equivalents for them.
func fetchOktaAppUsers(ctx context.Context, oktaClient api.Client, appID string, convertUser appUserConverter, logger *slog.Logger) (map[string]types.User, error) {
	result := map[string]types.User{}
	err := oktaClient.IterateAppUsers(ctx, oktaAppID(appID), func(oau *okta.AppUser) error {
		if oau == nil {
			logger.WarnContext(ctx, "Skipping missing AppUser")
			return nil
		}
		logger := logger.With(
			"user_name", oau.ExternalId,
			"okta_user_id", oau.Id,
		)
		logger.DebugContext(ctx, "Processing Okta AppUser")

		teleportUser, err := convertUser(oau)
		if err != nil {
			logger.WarnContext(ctx, "Failed converting appuser", "error", err)
			return nil
		}
		result[teleportUser.GetName()] = teleportUser
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err, "enumerating Okta app users")
	}
	return result, nil
}

// listTeleportUsers lists the teleport users that are managed by the Okta
// integration. Users for individual integrations are differentiated by the
// `userOrgURL`.
func listTeleportUsers(ctx context.Context, userSvc ReconcilerAccessPoint, userOrgURL string) (map[string]types.User, error) {
	result := map[string]types.User{}

	users, err := userSvc.GetUsers(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err, "listing teleport okta users")
	}

	isOktaUserInOrg := MatchByLabels[types.User](userOrgURL)
	for _, user := range users {
		// Filter out non-okta-origin users
		if !isOktaUserInOrg(user) {
			continue
		}

		result[user.GetName()] = user
	}

	return result, nil
}

// userReconcilerConfig holds the caller-supplied information needed to create a
// userReconciler.
type userReconcilerConfig struct {
	clusterName string
	teleportAP  ReconcilerAccessPoint
	userOrgURL  string
	emitter     apievents.Emitter
	clock       clockwork.Clock
	logger      *slog.Logger
}

// CheckAndSetDefaults validates the config, supplying defaults as necessary
func (cfg *userReconcilerConfig) CheckAndSetDefaults() error {
	if cfg.clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}

	if cfg.teleportAP == nil {
		return trace.BadParameter("missing access point")
	}

	if cfg.userOrgURL == "" {
		return trace.BadParameter("missing Okta org url")
	}

	if cfg.logger == nil {
		cfg.logger = slog.With(teleport.ComponentKey, eteleport.ComponentOkta)
	}

	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}

	return nil
}

// userSyncStats holds the statistics of an on-flight user synchronize operation
type userSyncStats struct {
	preSyncTotal int
	created      int
	modified     int
	deleted      int
}

// total returns the computed, post-sync total
func (stats *userSyncStats) total() int {
	return stats.preSyncTotal + stats.created - stats.deleted
}

// userReconciler reconciles Teleport users with an upstream Okta organization,
// creating, updating and deleting Teleport users as necessary.
type userReconciler struct {
	cfg           userReconcilerConfig
	backend       *services.Reconciler[types.User]
	stats         *userSyncStats
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
			Logger:              cfg.logger,
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
			r.cfg.logger.WarnContext(ctx, "User already exists in Teleport and was not imported",
				"user_name", oktaUser.GetName(),
				"okta_user_id", oktaUserID,
			)
			return nil
		}

		return trace.Wrap(err)
	}

	// Unlocking should be also called during user creation.
	// Deactivated user is not returned from Okta v1/users API thus after deactivation will be locked and removed from
	// Teleport. But if user will be activated again after a user was deleted from Teleport backend
	// we need to unlock all okta locks during user creation.
	if !UserHasLockableStatus(oktaUser) {
		reasons := []string{LockReasonDeleted, LockReasonDeactivated, LockReasonSuspended}
		if err := UnlockUser(ctx, oktaUser, reasons, r.cfg.userOrgURL, r.cfg.teleportAP); err != nil {
			return trace.Wrap(err)
		}
	}

	// log the user creation
	r.stats.created += 1

	return nil
}

// updateTeleportUser is the `OnUpdate` delegate for the inner reconciler
func (r *userReconciler) updateTeleportUser(ctx context.Context, newUser, oldUser types.User) error {
	if _, err := r.cfg.teleportAP.UpdateUser(ctx, newUser); err != nil {
		return trace.Wrap(err, "updating user %q", newUser.GetName())
	}

	r.stats.modified += 1

	oldUserLocked := UserHasLockableStatus(oldUser)
	newUserLocked := UserHasLockableStatus(newUser)
	transitionToLock := !oldUserLocked && newUserLocked
	transitionOutOfLock := oldUserLocked && !newUserLocked

	switch {
	case transitionToLock:
		// Create an okta lock on the user
		if _, err := r.lockUser(ctx, newUser, LockReasonSuspended); err != nil {
			return trace.Wrap(err, "locking on user %q", newUser.GetName())
		}

	case transitionOutOfLock:
		if err := UnlockUser(ctx, newUser, []string{LockReasonSuspended}, r.cfg.userOrgURL, r.cfg.teleportAP); err != nil {
			return trace.Wrap(err)
		}
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
	if _, err := r.lockUser(ctx, user, LockReasonDeleted); err != nil {
		return trace.Wrap(err)
	}
	r.stats.deleted += 1

	// Do the actual deletion
	if err := r.cfg.teleportAP.DeleteUser(ctx, user.GetName()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r *userReconciler) createSyncEvent(reconcilerErr error) *apievents.OktaUserSync {
	event := &apievents.OktaUserSync{
		Metadata: apievents.Metadata{
			Type:        events.OktaUserSyncEvent,
			Code:        events.OktaUserSyncSuccessCode,
			ClusterName: r.cfg.clusterName,
		},
		Status: apievents.Status{
			Success: true,
		},
	}

	if reconcilerErr != nil {
		event.Code = events.OktaUserSyncFailureCode
		event.Status = apievents.Status{
			Error:   reconcilerErr.Error(),
			Success: false,
		}

		return event
	}

	event.OrgUrl = r.cfg.userOrgURL
	event.NumUsersCreated = int32(r.stats.created)
	event.NumUsersDeleted = int32(r.stats.deleted)
	event.NumUsersModified = int32(r.stats.modified)
	event.NumUsersTotal = int32(r.stats.total())

	return event
}

// PreserveUserMetadata copies any metadata that needs to be preserved across an
// update from src to dst.
func PreserveUserMetadata(dst, src types.User) {
	dst.SetRevision(src.GetRevision())
	dst.SetCreatedBy(src.GetCreatedBy())
	dst.SetWeakestDevice(src.GetWeakestDevice())
	dst.SetPasswordState(src.GetPasswordState())
}

// reconcileUsers pulls the user list from an upstream okta organization and
// updates teleport users to match; creating, updating and deleting teleport
// users as needed. Reconciliation is strictly one-way; no attempts are made
// to update the upstream Okta organization.
func (r *userReconciler) reconcileUsers(ctx context.Context, oktaUsers, teleportUsers map[string]types.User) (*userSyncStats, error) {
	// Stash the supplied resource maps where the inner reconciler will be able
	// to find them
	r.oktaUsers = oktaUsers
	r.teleportUsers = teleportUsers
	r.stats = &userSyncStats{preSyncTotal: len(teleportUsers)}

	defer func() {
		r.teleportUsers = nil
		r.oktaUsers = nil
		r.stats = nil
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

		PreserveUserMetadata(oktaUser, teleportUser)
	}

	// Run the reconciliation
	reconcileErr := r.backend.Reconcile(ctx)

	// Emit an API event marking success or failure
	event := r.createSyncEvent(reconcileErr)
	if eventErr := r.cfg.emitter.EmitAuditEvent(ctx, event); eventErr != nil {
		r.cfg.logger.WarnContext(ctx, "Unable to emit audit event", "error", eventErr)
	}

	return r.stats, trace.Wrap(reconcileErr)
}

// LockParams holds the parameters required for creating an Okta-managed lock on
// a given user
type LockParams struct {
	User     types.User
	Reason   string
	Message  string
	OrgURL   string
	Clock    clockwork.Clock
	LocksSvc LocksService
	Logger   *slog.Logger
}

// CheckAndSetDefaults validates the LockParams values, supplying defaults if
// necessary.
func (p *LockParams) CheckAndSetDefaults() error {
	if p.User == nil {
		return trace.BadParameter("missing user")
	}

	if p.Reason == "" {
		return trace.BadParameter("missing lock reason")
	}

	if p.OrgURL == "" {
		return trace.BadParameter("missing target URL")
	}

	if p.LocksSvc == nil {
		return trace.BadParameter("missing locks service")
	}

	if p.Logger == nil {
		p.Logger = slog.With(teleport.ComponentKey, eteleport.ComponentOkta)
	}

	if p.Clock == nil {
		p.Clock = clockwork.NewRealClock()
	}

	return nil
}

// LockUser creates an okta-managed lock on a given user
func LockUser(ctx context.Context, args LockParams) (types.Lock, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	msg := args.Message
	if msg == "" {
		status, _ := args.User.GetLabel(eteleport.OktaUserStatusLabel)
		msg = fmt.Sprintf("Okta user %q is %s", args.User.GetName(), status)
	}

	expiry := args.Clock.Now().Add(lockTTL)
	l := &types.LockV2{
		Metadata: types.Metadata{
			Name: uuid.NewString(),
			Labels: map[string]string{
				types.OriginLabel:             types.OriginOkta,
				eteleport.OktaOrgURLLabel:     args.OrgURL,
				eteleport.OktaLockReasonLabel: args.Reason,
			},
			Expires: &expiry,
		},
		Spec: types.LockSpecV2{
			Message: msg,
			Target: types.LockTarget{
				User: args.User.GetName(),
			},
			CreatedBy: types.OriginOkta,
			Expires:   nil,
		},
	}

	if err := l.CheckAndSetDefaults(); err != nil {
		args.Logger.ErrorContext(ctx, "setting lock defaults", "error", err)
		return nil, trace.Wrap(err, "setting lock defaults")
	}

	args.Logger.DebugContext(ctx, "Locking user",
		"user_name", args.User.GetName(),
		"lock_name", l.GetName(),
	)

	if err := args.LocksSvc.UpsertLock(ctx, l); err != nil {
		return nil, trace.Wrap(err, "locking user %q", args.User.GetName())
	}

	return l, nil
}

// lockUser creates a Teleport lock targeting the supplied user. This will cause
// any open sessions held by that user to be immediately terminated.
func (r *userReconciler) lockUser(ctx context.Context, user types.User, reason string) (types.Lock, error) {
	return LockUser(ctx, LockParams{
		User:     user,
		Reason:   reason,
		OrgURL:   r.cfg.userOrgURL,
		Clock:    r.cfg.clock,
		LocksSvc: r.cfg.teleportAP,
		Logger:   r.cfg.logger,
	})
}

// UnlockUser deletes any Okta-managed locks on the target Teleport user. The
// deleted locks are filtered by the lock reason, so a request to delete
// `Suspended` locks will not delete `Deleted` locks.
func UnlockUser(ctx context.Context, user types.User, reasons []string, orgURL string, locksSvc LocksService) error {
	locks, err := getOktaLocksForUser(ctx, user, reasons, orgURL, locksSvc)
	if err != nil {
		return trace.Wrap(err, "fetching locks on user %q", user.GetName())
	}

	var lockErrors []error
	for _, lock := range locks {
		err := locksSvc.DeleteLock(ctx, lock.GetName())
		if err != nil && !trace.IsNotFound(err) {
			lockErrors = append(lockErrors,
				trace.Wrap(err,
					"deleting lock %s on user %q", lock.GetName(), user.GetName()))
		}
	}

	return trace.NewAggregate(lockErrors...)
}

// getOktaLocksForUser fetches all of the okta-created locks applied to the
// supplied user.
func getOktaLocksForUser(ctx context.Context, user types.User, reasons []string, orgURL string, locksSvc LocksService) ([]types.Lock, error) {
	allLocks, err := locksSvc.GetLocks(ctx, true /* in force only */, types.LockTarget{User: user.GetName()})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var oktaLocks []types.Lock
	for _, lock := range allLocks {
		if isOktaLock(lock, orgURL, reasons) {
			oktaLocks = append(oktaLocks, lock)
		}
	}

	return oktaLocks, nil
}

// lockableStatuses is the set of Okta user statuses from https://help.okta.com/en-us/content/topics/users-groups-profiles/usgp-end-user-states.htm
var lockableStatuses = map[string]struct{}{
	userStatusSuspended:     {},
	userStatusDeprovisioned: {},
}

// UserHasLockableStatus test if the supplied user is in an Okta state where
// their account should be locked
func UserHasLockableStatus(user types.User) bool {
	status, _ := user.GetLabel(eteleport.OktaUserStatusLabel)
	_, isLockable := lockableStatuses[strings.ToUpper(status)]
	return isLockable
}

// isOktaLock identifies a Teleport lock as belonging to *this* Okta service by
// examining the lock origin and Okta services.
func isOktaLock(lock types.Lock, orgUrl string, reasons []string) bool {
	if lock.Origin() != types.OriginOkta {
		return false
	}

	if url, _ := lock.GetLabel(eteleport.OktaOrgURLLabel); url != orgUrl {
		return false
	}

	if lr, _ := lock.GetLabel(eteleport.OktaLockReasonLabel); !slices.Contains(reasons, lr) {
		return false
	}

	return true
}
