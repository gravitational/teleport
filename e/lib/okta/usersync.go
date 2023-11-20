package okta

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
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
}

// userConverter is a function that can convert an Okta user into a Teleport
// user.
type userConverter func(*okta.User) (types.User, error)

// fetchOktaUsers fetches users from the upstream okta service and creates
// candidate Teleport user equivalents for them.
func fetchOktaUsers(ctx context.Context, oktaClient oktaClient, convertUser userConverter, log logrus.FieldLogger) (types.ResourcesWithLabelsMap, error) {
	result := types.ResourcesWithLabelsMap{}
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
func listTeleportUsers(ctx context.Context, userSvc ReconcilerAccessPoint, userOrgURL string, log logrus.FieldLogger) (types.ResourcesWithLabelsMap, error) {
	result := types.ResourcesWithLabelsMap{}

	users, err := userSvc.GetUsers(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err, "listing teleport okta users")
	}

	for _, user := range users {
		// Filter out non-okta-origib users
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

	return nil
}

// userReconciler reconciles Teleport users with an upstream Okta organization,
// creating, updating and deleting Teleport users as necessary.
type userReconciler struct {
	cfg           userReconcilerConfig
	backend       *services.Reconciler
	teleportUsers types.ResourcesWithLabelsMap
	oktaUsers     types.ResourcesWithLabelsMap
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
		services.ReconcilerConfig{
			Matcher: func(r types.ResourceWithLabels) bool {
				_, ok := r.(types.User)
				return ok
			},
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
func (r *userReconciler) getTeleportUsers() types.ResourcesWithLabelsMap {
	return r.teleportUsers
}

// getTeleportUsers supplies the "new" Okta-derived user resources to the inner
// Reconciler
func (r *userReconciler) getOktaUsers() types.ResourcesWithLabelsMap {
	return r.oktaUsers
}

// createTeleportUser is the `OnCreate` delegate for the inner reconciler
func (r *userReconciler) createTeleportUser(ctx context.Context, res types.ResourceWithLabels) error {
	oktaUser, ok := res.(types.User)
	if !ok {
		return trace.BadParameter("res must be types.User")
	}

	_, err := r.cfg.teleportAP.CreateUser(ctx, oktaUser)
	if err == nil {
		return nil
	}

	if trace.IsAlreadyExists(err) {
		oktaUserID, ok := oktaUser.GetLabel(eteleport.OktaUserIDLabel)
		if !ok {
			oktaUserID = "<UNKNOWN>"
		}

		r.cfg.log.Warningf("User %q already exists in Teleport. Okta user %s was not imported.",
			oktaUser.GetName(), oktaUserID)
		return nil
	}

	return err
}

// updateTeleportUser is the `OnUpdate` delegate for the inner reconciler
func (r *userReconciler) updateTeleportUser(ctx context.Context, res types.ResourceWithLabels) error {
	user, ok := res.(types.User)
	if !ok {
		return trace.BadParameter("res must be types.User")
	}

	if _, err := r.cfg.teleportAP.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err, "updating user %q", user.GetName())
	}
	return nil
}

// deleteTeleportUser is the `OnDelete` delegate for the inner reconciler
func (r *userReconciler) deleteTeleportUser(ctx context.Context, res types.ResourceWithLabels) error {
	user, ok := res.(types.User)
	if !ok {
		return trace.BadParameter("res must be types.User")
	}

	if err := r.cfg.teleportAP.DeleteUser(ctx, user.GetName()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// reconcileUsers pulls the user list from an upstream okta organization and
// updates teleport users to match; creating, updating and deleting teleport
// users as needed. Reconciliation is strictly one-way; no attempts are made
// to update the upstream Okta organization.
func (r *userReconciler) reconcileUsers(ctx context.Context, oktaUsers, teleportUsers types.ResourcesWithLabelsMap) error {
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
	for username, teleportResource := range teleportUsers {
		oktaResource, ok := oktaUsers[username]
		if !ok {
			continue
		}

		teleportUser, ok := teleportResource.(types.User)
		if !ok {
			r.cfg.log.Warnf("teleport user must be types.User")
			continue
		}

		oktaUser, ok := oktaResource.(types.User)
		if !ok {
			r.cfg.log.Warnf("okta user must be types.User")
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
