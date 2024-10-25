package okta

import (
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"maps"
	"sort"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

const (
	userAssignmentCreatorSource = "user-assignment-creator"
)

// UserAssignmentCreatorAccessPoint is a client that consists of only the interfaces
// needed for the UserAssignmentCreator.
type UserAssignmentCreatorAccessPoint interface {
	services.RoleGetter

	// GetLocks gets all/in-force locks that match at least one of the targets when specified.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)

	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

	// ListUserGroups returns a paginated list of all user group resources.
	ListUserGroups(context.Context, int, string) ([]types.UserGroup, string, error)

	// GetOktaAssignment returns the specified Okta assignment resources.
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)

	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)

	// CreateOktaAssignment creates a new Okta assignment resource.
	CreateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)

	// UpdateOktaAssignment updates an existing Okta assignment resource.
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)

	// GetUserOrLoginState will return the given user or the login state associated with the user.
	GetUserOrLoginState(ctx context.Context, username string) (services.UserState, error)
}

// UserAssignmentCreatorConfig is the configuration for the UserAssignmentCreator.
type UserAssignmentCreatorConfig struct {
	// Log is the logger for the UserAssignmentCreator.
	Log *logrus.Entry

	// Clock is the clock to use for the UserAssignmentCreator.
	Clock clockwork.Clock

	// OktaConnected is a utility that will detect if an Okta service is connected.
	OktaConnected *connected.OktaConnected

	// ClusterName is the name of the cluster.
	ClusterName string

	// AccessPoint is the access point for the user assignment creator.
	AccessPoint UserAssignmentCreatorAccessPoint

	// PrintDiffs will print user diffs if set to true.
	PrintDiffs bool
}

func (c *UserAssignmentCreatorConfig) CheckAndSetDefaults() error {
	if c.ClusterName == "" {
		return trace.BadParameter("cluster name is missing")
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}

	if c.OktaConnected == nil {
		return trace.BadParameter("okta connected is missing")
	}

	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, eteleport.ComponentOktaUserAssignmentCreator)
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

// UserAssignmentCreator will create an assignment on user login to ensure that
// the permissions the user has within Teleport are appropriately reflected
// in Okta.
type UserAssignmentCreator struct {
	log         *logrus.Entry
	clock       clockwork.Clock
	clusterName string
	accessPoint UserAssignmentCreatorAccessPoint
	accessState services.AccessState
	connected   *connected.OktaConnected

	// hash will be used to calculate the name of the assignment to create.
	hash          crypto.Hash
	groupPageSize int
	appPageSize   int
	printDiffs    bool
}

// NewUserAssignmentCreator creates a new user assignment creator.
func NewUserAssignmentCreator(config UserAssignmentCreatorConfig) (*UserAssignmentCreator, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	creator := &UserAssignmentCreator{
		log:         config.Log,
		clock:       config.Clock,
		clusterName: config.ClusterName,
		accessPoint: config.AccessPoint,
		connected:   config.OktaConnected,

		// We'll use an access state with MFAVerified to true because, for the RBAC calculations
		// made here, we don't need to use MFA.
		accessState: services.AccessState{
			MFAVerified: true,
		},
		hash:        crypto.SHA256,
		appPageSize: defaults.DefaultChunkSize,
		printDiffs:  config.PrintDiffs,
	}

	return creator, nil
}

// OnLogin will be run on login and will reconcile the user's current permissions to Okta assignment
// access.
func (u *UserAssignmentCreator) OnLogin(ctx context.Context, user types.User) error {
	// If no Okta service is connected, return immediately. Anything that is missed will be caught
	// once the Okta service connects and the usermonitor re-runs.
	if !u.connected.IsConnected(ctx) {
		return nil
	}

	userState, err := u.accessPoint.GetUserOrLoginState(ctx, user.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	// No need to make assignments for non-SSO users.
	if userState.GetUserType() != types.UserTypeSSO {
		return nil
	}

	var groups []string
	var apps []string

	// Only calculate access if a user has no locks in force.
	locks, err := u.accessPoint.GetLocks(ctx, true, types.LockTarget{
		User: userState.GetName(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(locks) != 0 {
		// If there are locks in force, we'll skip the okta assignment creation/deletion.
		// A Locked user still need to have preserved all assignments after unlocking.
		return nil
	}

	// It should be okay to get the access info from the user state here because we're only concerned about
	// the permissions tied to the user and associated permissions granted by access lists.
	accessInfo := services.AccessInfoFromUserState(userState)
	accessChecker, err := services.NewAccessChecker(accessInfo, u.clusterName, u.accessPoint)
	if err != nil {
		return trace.Wrap(err)
	}

	groups, err = u.groupTargets(ctx, accessChecker)
	if err != nil {
		return trace.Wrap(err, "listing user groups for Okta access calculation")
	}

	apps, err = u.appServerTargets(ctx, accessChecker)
	if err != nil {
		return trace.Wrap(err, "listing app servers for Okta access calculation")
	}

	assignmentName, err := uacAssignmentName(u.hash, userState.GetName(), groups, apps)
	if err != nil {
		return trace.Wrap(err, "creating user OktaAssignment name")
	}

	var newAssignment types.OktaAssignment

	// Only create an assignment if there are groups and apps to add to it.
	if len(groups) != 0 || len(apps) != 0 {
		// The Okta assignment already exists, so skip any further processing.
		foundAssignment, err := u.accessPoint.GetOktaAssignment(ctx, assignmentName)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "finding Okta assignment for user %s", userState.GetName())
		}

		// The current assignment is still active, so we'll return.
		if foundAssignment != nil && foundAssignment.GetCleanupTime().IsZero() {
			return nil
		}

		// The Okta assignment doesn't exist, so let's create the new one.
		newAssignment, err = u.newOktaAssignment(ctx, assignmentName, userState.GetName(), groups, apps)
		if err != nil {
			return trace.Wrap(err, "creating the new Okta assignment")
		}

		// Create the new assignment in the backend and remove the old assignments.
		if foundAssignment != nil {
			// If the found assignment is present but it's finalized, it means we're restoring duplicate
			// access. We'll update the Okta assignment to reflect the new state.
			_, err = u.accessPoint.UpdateOktaAssignment(ctx, newAssignment)
			if err != nil {
				return trace.Wrap(err, "update the new Okta assignment in the backend")
			}
		} else {
			_, err = u.accessPoint.CreateOktaAssignment(ctx, newAssignment)
			if err != nil {
				return trace.Wrap(err, "creating the new Okta assignment in the backend")
			}
		}
	}

	// The Okta assignment doesn't exist, so find the old reconciler assignment for this user.
	oldAssignments, err := u.findOldOktaAssignments(ctx, userState.GetName())
	if err != nil {
		return trace.Wrap(err, "finding old Okta assignments")
	}

	for _, oldAssignment := range oldAssignments {
		// Only retire old assignments if they don't match the name of the new assignment.
		if oldAssignment.GetName() != assignmentName {
			oldAssignment.SetCleanupTime(u.clock.Now())
			if newAssignment != nil {
				neededTargets := newAssignment.GetTargets()
				// Remove the targets that are being used in the new assignment.
				// Let's say that user still have access to target A and B but old assigment has A, B, C, D.
				// In order to prevent the cleanup of A and B, we need to remove them from the old assignment.
				if err := removedUsedTargetsFromOldAssignment(neededTargets, oldAssignment); err != nil {
					return trace.Wrap(err, "removing used targets from old assignment %s", oldAssignment.GetName())
				}
			}
			if _, err := u.accessPoint.UpdateOktaAssignment(ctx, oldAssignment); err != nil {
				return trace.Wrap(err, "cleaning up old assignment %s", oldAssignment.GetName())
			}
		}
	}

	// If debug is enabled, print out the diff of the old vs. new assignments.
	if logrus.IsLevelEnabled(logrus.DebugLevel) && u.printDiffs {
		newGroups, newApps, removedGroups, removedApps := assignmentDiff(newAssignment, oldAssignments...)

		u.log.Debugf("New groups for user %s: %v", userState.GetName(), newGroups)
		u.log.Debugf("New apps for user %s: %v", userState.GetName(), newApps)
		u.log.Debugf("Removed groups for user %s: %v", userState.GetName(), removedGroups)
		u.log.Debugf("Removed apps for user %s: %v", userState.GetName(), removedApps)
	}

	return nil
}

func removedUsedTargetsFromOldAssignment(usedTargets []types.OktaAssignmentTarget, old types.OktaAssignment) error {
	targetKey := func(target types.OktaAssignmentTarget) string {
		return fmt.Sprintf("%s:%s", target.GetTargetType(), target.GetID())
	}
	usedTargetsSet := map[string]struct{}{}
	for _, v := range usedTargets {
		usedTargetsSet[targetKey(v)] = struct{}{}
	}
	var result []*types.OktaAssignmentTargetV1
	for _, target := range old.GetTargets() {
		if _, ok := usedTargetsSet[targetKey(target)]; !ok {
			tv1, ok := target.(*types.OktaAssignmentTargetV1)
			if !ok {
				return trace.BadParameter("expected OktaAssignmentTargetV1, got %T", target)
			}
			result = append(result, tv1)
		}
		v1, ok := old.(*types.OktaAssignmentV1)
		if !ok {
			return trace.BadParameter("expected OktaAssignmentV1, got %T", old)
		}
		v1.Spec.Targets = result
	}
	return nil
}

// groupTargets returns the names of all Okta user groups that the user has access to.
func (u *UserAssignmentCreator) groupTargets(ctx context.Context, accessChecker services.AccessChecker) ([]string, error) {
	targets := map[string]struct{}{}

	// Page through the user groups.
	groups, nextKey, err := u.accessPoint.ListUserGroups(ctx, u.groupPageSize, "")

	for {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, group := range groups {
			// Only check Okta groups.
			if group.Origin() == types.OriginOkta {
				// If the user has access to the group, add it to the list of targets.
				if err := accessChecker.CheckAccess(group, u.accessState); err == nil {
					targets[group.GetName()] = struct{}{}
				} else if !trace.IsAccessDenied(err) {
					u.log.Errorf("Error checking access to group during login: %v", err)
				}
			}
		}

		if nextKey == "" {
			break
		}

		// Get the next page of results.
		groups, nextKey, err = u.accessPoint.ListUserGroups(ctx, u.groupPageSize, nextKey)
	}

	var out []string
	for k := range maps.Keys(targets) {
		out = append(out, k)
	}
	return out, nil
}

// groupTargets returns the names of all Okta app IDs that the user has access to.
func (u *UserAssignmentCreator) appServerTargets(ctx context.Context, accessChecker services.AccessChecker) ([]string, error) {
	targets := map[string]struct{}{}

	// Page through the app servers.
	resp, err := u.accessPoint.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        int32(u.appPageSize),
	})

	for {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, resource := range resp.Resources {
			// Only check Okta apps.
			if resource.Origin() == types.OriginOkta {
				appServer, ok := resource.(types.AppServer)

				if !ok {
					u.log.Errorf("Expected AppServer, got %T", resource)
					continue
				}
				app := appServer.GetApp()
				// If the user has access to the app, extra the Okta app label and add it to the list of targets.
				if err := accessChecker.CheckAccess(app, u.accessState); err == nil {
					// We're deduplicating here because app servers are not unique depending on the heartbeat/backend/etc.,
					// so we're making sure that we're not creating duplicate entries for the same apps.
					targets[app.GetName()] = struct{}{}
				} else if !trace.IsAccessDenied(err) {
					u.log.Errorf("Error checking access to application during login: %v", err)
				}
			}
		}

		if resp.NextKey == "" {
			break
		}

		// Get the next page of results.
		resp, err = u.accessPoint.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindAppServer,
			StartKey:     resp.NextKey,
			Limit:        int32(u.appPageSize),
		})
	}

	var out []string
	for k := range maps.Keys(targets) {
		out = append(out, k)
	}
	return out, nil
}

// newOktaAssignment will create an Okta assignment resource corresponding to the groups and apps given. The groups and apps
// are expected to be Okta IDs.
func (u *UserAssignmentCreator) newOktaAssignment(ctx context.Context, assignmentName, username string,
	groups []string, apps []string) (types.OktaAssignment, error) {
	targets := make([]*types.OktaAssignmentTargetV1, 0, len(groups)+len(apps))

	for _, group := range groups {
		targets = append(targets, &types.OktaAssignmentTargetV1{
			Type: types.OktaAssignmentTargetV1_GROUP,
			Id:   group,
		})
	}

	for _, app := range apps {
		targets = append(targets, &types.OktaAssignmentTargetV1{
			Type: types.OktaAssignmentTargetV1_APPLICATION,
			Id:   app,
		})
	}

	assignment, err := types.NewOktaAssignment(types.Metadata{
		Name: assignmentName,
		Labels: map[string]string{
			eteleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
		},
	}, types.OktaAssignmentSpecV1{
		User:           username,
		Targets:        targets,
		Status:         types.OktaAssignmentSpecV1_PENDING,
		LastTransition: u.clock.Now(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return assignment, nil
}

// findOldOktaAssignments will find the old active Okta assignment(s) intended for this user.
func (u *UserAssignmentCreator) findOldOktaAssignments(ctx context.Context, username string) (types.OktaAssignments, error) {
	var foundAssignments types.OktaAssignments

	assignments, nextKey, err := u.accessPoint.ListOktaAssignments(ctx, 0, "")
	for {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, assignment := range assignments {
			sourceLabel, ok := assignment.GetLabel(eteleport.OktaAssignmentSourceLabel)
			if !ok {
				u.log.Debugf("No source label for assignment %s, skipping", assignment.GetName())
				continue
			}

			// If this action belongs to this user, it's made by the user assignment creator, and it's not all cleaned up,
			// this is an old active assignment.
			if assignment.GetUser() == username && sourceLabel == userAssignmentCreatorSource && !assignment.IsFinalized() {
				// It's not expected that we have more than one assignment created by the user assignment creator, but in the
				// interests of being comprehensive, we'll check for more than one.
				foundAssignments = append(foundAssignments, assignment)
			}
		}

		if nextKey == "" {
			break
		}

		assignments, nextKey, err = u.accessPoint.ListOktaAssignments(ctx, 0, nextKey)
	}

	return foundAssignments, nil
}

// assignmentDiff returns the new groups/apps and removed groups/apps from Okta.
func assignmentDiff(newAssignment types.OktaAssignment, oldAssignments ...types.OktaAssignment) (newGroups,
	newApps, removedGroups, removedApps []string) {
	newGroupsMap := map[string]struct{}{}
	newAppsMap := map[string]struct{}{}
	removedGroupsMap := map[string]struct{}{}
	removedAppsMap := map[string]struct{}{}

	// register all new targets in the new maps.
	if newAssignment != nil {
		for _, target := range newAssignment.GetTargets() {
			switch target.GetTargetType() {
			case constants.OktaAssignmentTargetGroup:
				newGroupsMap[target.GetID()] = struct{}{}
			case constants.OktaAssignmentTargetApplication:
				newAppsMap[target.GetID()] = struct{}{}
			}
		}
	}

	for _, oldAssignment := range oldAssignments {
		for _, target := range oldAssignment.GetTargets() {
			switch target.GetTargetType() {
			case constants.OktaAssignmentTargetGroup:
				// If the group is present in the old assignment, it's not being created or removed,
				// so remove it from the new map. If it's not present in the new assignment, it's being
				// removed, so add it to the removed map.
				if _, ok := newGroupsMap[target.GetID()]; ok {
					delete(newGroupsMap, target.GetID())
				} else {
					removedGroupsMap[target.GetID()] = struct{}{}
				}
			case constants.OktaAssignmentTargetApplication:
				// If the app is present in the old assignment, it's not being created or removed,
				// so remove it from the new map. If it's not present in the new assignment, it's being
				// removed, so add it to the removed map.
				if _, ok := newAppsMap[target.GetID()]; ok {
					delete(newAppsMap, target.GetID())
				} else {
					removedAppsMap[target.GetID()] = struct{}{}
				}
			}
		}
	}

	// Put these all into sorted lists.
	for newGroup := range newGroupsMap {
		newGroups = append(newGroups, newGroup)
	}
	for newApp := range newAppsMap {
		newApps = append(newApps, newApp)
	}
	for removedGroup := range removedGroupsMap {
		removedGroups = append(removedGroups, removedGroup)
	}
	for removedApp := range removedAppsMap {
		removedApps = append(removedApps, removedApp)
	}

	sort.Strings(newGroups)
	sort.Strings(newApps)
	sort.Strings(removedGroups)
	sort.Strings(removedApps)

	return newGroups, newApps, removedGroups, removedApps
}

// uacAssignmentName returns an assignment name based on the targets, which are the list of user groups
// and applications this user has access to.
func uacAssignmentName(hash crypto.Hash, user string, groups, apps []string) (string, error) {
	// Sort the lists so that the same targets will result in the same ID.
	sort.Strings(groups)
	sort.Strings(apps)

	hasher := hash.New()
	// Write the user to the hash.
	_, err := hasher.Write([]byte(user + "/"))
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Write the groups to the hash.
	for _, group := range groups {
		_, err := hasher.Write([]byte(group + "/"))
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	// Write the apps to the hash.
	for _, app := range apps {
		_, err := hasher.Write([]byte(app + "/"))
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	// Get the hashed ID.
	hashedID := hasher.Sum(nil)

	// Encode the string so that it can sit in the backend.
	return base64.RawURLEncoding.EncodeToString(hashedID), nil
}
