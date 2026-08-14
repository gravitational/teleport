package okta

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"golang.org/x/sync/singleflight"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/set"
)

const (
	// userListTimeout is the maximum amount of time to spend listing Okta users.
	// It is long because listing a page of 500 users using the regular API
	// (not Skinny Users Endpoints) may take minutes.
	userListTimeout time.Duration = 10 * time.Minute
)

// assignmentClient is a caching Okta client that will keep track of of Okta
// state while assignments are processed.  When querying for assignment state
// for a group or app, if the current list of assignments to the target has not
// yet been retrieved from Okta, it will be retrieved upon request. This client
// should be discarded at the end of an assignment loop or singular assignment run.
type assignmentClient struct {
	logger     *slog.Logger
	oktaClient oktaapi.Interface

	// usersMu is used to protect initialization of users, deactivatedUsers and suspendedUsers.
	usersMu sync.Mutex
	// usersReady indicates whether users has been successfully populated.
	usersReady bool

	// Mapping of usernames to user IDs.
	// Initialized once by ensureUsers under usersMu lock, and read only when usersReady is true.
	users map[userName]oktaUserID

	// deactivatedUsers is the set of users reported deactivated by Okta API.
	deactivatedUsers set.Set[userName]
	// suspendedUsers is the set of users reported suspended by Okta API.
	suspendedUsers set.Set[userName]

	// Group membership.
	groups utils.SyncMap[oktaGroupID, set.Set[oktaUserID]]

	// Apps membership.
	apps utils.SyncMap[oktaAppID, set.Set[oktaapi.AppAssignment]]

	// syncSingleFlight is used to prevent multiple requests during listing user apps groups
	// assignments. Parallel calls will be collapsed in to one and all receive the same result.
	// Assumed that Okta GroupID and AppID  are exclusive uniq (Okta Group ID != Okta App ID)
	syncSingleFlight singleflight.Group
}

// newAssignmentClient will return a new assignment client.
func newAssignmentClient(log *slog.Logger, oktaClient oktaapi.Interface) *assignmentClient {
	return &assignmentClient{
		logger:     log,
		oktaClient: oktaClient,
	}
}

func (a *assignmentClient) getGroupAssignments(ctx context.Context, groupID oktaGroupID) (set.Set[oktaUserID], error) {
	// syncSingleFlight is used to prevent multiple requests during listing user groups assignments.
	// assignments are processed in parallel (See processAssignments function)
	// We need to make sure that we don't make multiple requests to Okta for the same group.
	// After the first request, the result is cached and returned for subsequent requests.
	key := fmt.Sprintf("group:%s", groupID)
	items, err, _ := a.syncSingleFlight.Do(key, func() (any, error) {
		cachedMembers, populated := a.groups.Load(groupID)
		if populated {
			return cachedMembers, nil
		}

		a.logger.DebugContext(ctx, "Refreshing group assignments", "group", groupID)
		members, err := a.oktaClient.GetGroupAssignments(ctx, groupID)
		if err != nil {
			return false, trace.Wrap(err)
		}
		cachedMembers = set.New[oktaUserID](members...)
		a.logger.DebugContext(ctx, "Found users assigned to group",
			"members", members,
			"group_id", groupID,
		)
		a.groups.Store(groupID, cachedMembers)
		return cachedMembers, nil
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}
	members, ok := items.(set.Set[oktaUserID])
	if !ok {
		return nil, trace.BadParameter("unexpected type %T returned", items)
	}
	return members, nil
}

// userAssignedToGroup will return true if the user is assigned to the group.
func (a *assignmentClient) userAssignedToGroup(ctx context.Context, username userName, groupID oktaGroupID) (bool, error) {
	userID, err := a.userID(ctx, username)
	if err != nil {
		return false, trace.Wrap(err)
	}

	members, err := a.getGroupAssignments(ctx, groupID)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// We check for group membership under the group map read lock. It's possible
	// Someone *may* have changed the membership set behind our back in the time
	// since we fetched it, but at least this way no on can change it while
	// we're in the middle of a read.

	ok := false
	a.groups.Read(func(_ map[oktaGroupID]set.Set[oktaUserID]) {
		ok = members.Contains(userID)
	})

	return ok, nil
}

// registerUserToGroup will register the user to the group.
func (a *assignmentClient) registerUserToGroup(ctx context.Context, username userName, groupID oktaGroupID) error {
	ok, err := a.userAssignedToGroup(ctx, username, groupID)
	if err != nil {
		return trace.Wrap(err)
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already registered.
	if ok {
		a.logger.DebugContext(ctx, "User is already assigned to group", "user_id", userID, "group_id", groupID)
		return nil
	}

	if err := a.oktaClient.AssignUserToGroup(ctx, userID, groupID); err != nil {
		return trace.Wrap(err)
	}

	a.logger.DebugContext(ctx, "User has been assigned to group", "user_id", userID, "group_id", groupID)

	// update our local cache to reflect the new assignment in the upstream
	// Okta org
	a.groups.Write(func(groups map[oktaGroupID]set.Set[oktaUserID]) {
		groups[groupID].Add(userID)
	})

	return nil
}

// unregisterUserFromGroup will unregister the user from the group.
func (a *assignmentClient) unregisterUserFromGroup(ctx context.Context, username userName, groupID oktaGroupID) error {
	ok, err := a.userAssignedToGroup(ctx, username, groupID)
	if err != nil {
		return trace.Wrap(err)
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	logger := a.logger.With("user_id", userID, "group_id", groupID)

	// Already unregistered.
	if !ok {
		logger.DebugContext(ctx, "User is already unassigned from group")
		return nil
	}

	if err := a.oktaClient.UnassignUserFromGroup(ctx, userID, groupID); err != nil {
		if oErr := (*oktaapi.OktaAPIValidationError)(nil); errors.As(err, &oErr) {
			// This is referring to Okta group rules:
			// https://help.okta.com/en-us/Content/Topics/users-groups-profiles/usgp-about-group-rules.htm
			logger.WarnContext(ctx, "Unable to remove user from group due to API validation exception. This membership is likely managed by Okta group rules. Proceeding as if this were successful.")
		} else if trace.IsNotFound(err) {
			logger.WarnContext(ctx, "Unable to remove user from group due because the group cannot be found in Okta. Proceeding as if this were successful.")
		} else {
			return trace.Wrap(err)
		}
	} else {
		logger.DebugContext(ctx, "User has been unassigned from groups")
	}

	// Update the local group cache to match the upstream Okta organization
	a.groups.Write(func(groups map[oktaGroupID]set.Set[oktaUserID]) {
		groups[groupID].Remove(userID)
	})

	return nil
}

func (a *assignmentClient) getUserAssignedToApp(ctx context.Context, appID oktaAppID) (set.Set[oktaapi.AppAssignment], error) {
	// syncSingleFlight is used to prevent multiple requests during listing user apps assignments.
	// assignments are processed in parallel (See processAssignments function)
	// We need to make sure that we don't make multiple requests to Okta for the same app.
	// After the first request, the result is cached and returned for subsequent requests.
	key := fmt.Sprintf("app:%s", appID)
	items, err, _ := a.syncSingleFlight.Do(key, func() (any, error) {
		cached, populated := a.apps.Load(appID)
		if populated {
			return cached, nil
		}
		a.logger.DebugContext(ctx, "Refreshing app assignments", "app_id", appID)
		items, err := a.oktaClient.GetAppAssignments(ctx, appID)
		if err != nil {
			return false, trace.Wrap(err)
		}
		cached = set.New[oktaapi.AppAssignment]()
		for _, member := range items {
			cached.Add(member)
		}
		a.logger.DebugContext(ctx, "Found users assigned to app",
			"members", len(cached),
			"app_id", appID,
		)

		a.apps.Store(appID, cached)
		return cached, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members, ok := items.(set.Set[oktaapi.AppAssignment])
	if !ok {
		return nil, trace.BadParameter("unexpected type %T returned", items)
	}
	return members, nil
}

// userAssignedToApp will return true if the user is assigned to the app.
func (a *assignmentClient) userAssignedToApp(ctx context.Context, username userName, appID oktaAppID) (bool, error) {
	userID, err := a.userID(ctx, username)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the app entry hasn't yet been populated, populate it.
	assignments, err := a.getUserAssignedToApp(ctx, appID)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// We check for app assignment under the application map read lock. It's
	// possible someone *may* have changed the assignment set behind our back
	// in the time since we fetched it, but at least this way no on can change
	// it while we're in the middle of a read.

	assignedToGroup := false
	assignedToApp := false
	a.apps.Read(func(_ map[oktaAppID]set.Set[oktaapi.AppAssignment]) {
		assignedToGroup = assignments.Contains(oktaapi.AppAssignment{UserID: string(userID), Scope: oktaapi.GroupScope})
		assignedToApp = assignments.Contains(oktaapi.AppAssignment{UserID: string(userID), Scope: oktaapi.UserScope})
	})

	return assignedToGroup || assignedToApp, nil
}

// registerUserToApp will register the user to the app.
func (a *assignmentClient) registerUserToApp(ctx context.Context, username userName, appID oktaAppID) error {
	ok, err := a.userAssignedToApp(ctx, username, appID)
	if err != nil {
		return trace.Wrap(err)
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already registered.
	if ok {
		a.logger.DebugContext(ctx, "User is already assigned to app", "user_id", userID, "app_id", appID)
		return nil
	}

	// Make update to upstream Okta
	if err := a.oktaClient.AssignUserToApplication(ctx, userID, appID); err != nil {
		return trace.Wrap(err)
	}

	// Update local cache
	a.logger.DebugContext(ctx, "User has been assigned to app", "user_id", userID, "app_id", appID)
	a.apps.Write(func(apps map[oktaAppID]set.Set[oktaapi.AppAssignment]) {
		apps[appID].Add(oktaapi.AppAssignment{UserID: string(userID), Scope: oktaapi.UserScope})
	})

	return nil
}

// unregisterUserFromGroup will unregister the user from the app.
func (a *assignmentClient) unregisterUserFromApp(ctx context.Context, username userName, appID oktaAppID) error {
	ok, err := a.userAssignedToApp(ctx, username, appID)
	if err != nil {
		return trace.Wrap(err)
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	logger := a.logger.With("user_id", userID, "app_id", appID)

	// Already unregistered.
	if !ok {
		logger.DebugContext(ctx, "User has already been unassigned from app")
		return nil
	}

	if err := a.oktaClient.UnassignUserFromApplication(ctx, userID, appID); err != nil {
		if oErr := (*oktaapi.OktaAPIValidationError)(nil); errors.As(err, &oErr) {
			// This is referring to Okta group rules:
			// https://help.okta.com/en-us/Content/Topics/users-groups-profiles/usgp-about-group-rules.htm
			logger.WarnContext(ctx, "Unable to remove user from application due to API validation exception. Proceeding as if this were successful.")
		} else if trace.IsNotFound(err) {
			logger.WarnContext(ctx, "Unable to remove user from application due because the application cannot be found in Okta. Proceeding as if this were successful.")
		} else {
			return trace.Wrap(err)
		}
	} else {
		logger.DebugContext(ctx, "User has been unassigned from app")
	}

	// Update local cache
	a.apps.Write(func(apps map[oktaAppID]set.Set[oktaapi.AppAssignment]) {
		apps[appID].Remove(oktaapi.AppAssignment{UserID: string(userID), Scope: oktaapi.UserScope})
	})

	return nil
}

// userID will return the Okta userID for the username.
func (a *assignmentClient) userID(ctx context.Context, username userName) (oktaUserID, error) {
	if err := a.ensureUsers(ctx); err != nil {
		return "", trace.Wrap(err)
	}

	uid, ok := a.users[username]
	if !ok {
		return "", trace.NotFound("unable to find ID for user %s", username)
	}

	return uid, nil
}

// ensureUsers builds the user lists and sets the usersReady flag once successful.
// This ensures that subsequent calls after an initial failure either error or return
// the full users listing.
func (a *assignmentClient) ensureUsers(ctx context.Context) error {
	a.usersMu.Lock()
	defer a.usersMu.Unlock()

	if a.usersReady {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, userListTimeout)
	defer cancel()

	var err error

	a.logger.DebugContext(ctx, "Refreshing organization user list")
	a.users, err = a.oktaClient.ListUsers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	deactivatedFilter := fmt.Sprintf(`status eq "%s"`, userStatusDeprovisioned)
	deactivatedUsers, err := a.oktaClient.ListUsers(ctx, query.WithFilter(deactivatedFilter))
	if err != nil {
		return trace.Wrap(err)
	}

	a.deactivatedUsers = set.New(slices.Collect(maps.Keys(deactivatedUsers))...)

	suspendedFilter := fmt.Sprintf(`status eq "%s"`, userStatusSuspended)
	suspendedUsers, err := a.oktaClient.ListUsers(ctx, query.WithFilter(suspendedFilter))
	if err != nil {
		return trace.Wrap(err)
	}

	a.suspendedUsers = set.New(slices.Collect(maps.Keys(suspendedUsers))...)

	if a.users == nil {
		a.users = make(map[oktaapi.UserName]oktaapi.OktaUserID)
	}
	// Assignment can be stale and refer to deactivated or suspended user.
	// userID function needs succeed to successfully process and clean the assignment.
	// We need to make sure that all assignments for deactivated and suspended users
	// are cleaned up before deleting okta assignment.
	maps.Copy(a.users, deactivatedUsers)
	// Suspended users are technically already in users, but since we re-fetch only
	// suspended users subsequently, update users to make sure both align.
	maps.Copy(a.users, suspendedUsers)

	a.usersReady = true

	return nil
}

// userExists checks whether an Okta user with the username exists.
func (a *assignmentClient) userExists(ctx context.Context, username userName) (bool, error) {
	_, err := a.userID(ctx, username)
	switch {
	case trace.IsNotFound(err):
		return false, nil
	case err != nil:
		return false, trace.Wrap(err)
	}

	return true, nil
}
