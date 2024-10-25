package okta

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common/set"
	"github.com/gravitational/teleport/lib/utils"
)

// assignmentClient is a caching Okta client that will keep track of of Okta
// state while assignments are processed.  When querying for assignment state
// for a group or app, if the current list of assignments to the target has not
// yet been retrieved from Okta, it will be retrieved upon request. This client
// should be discarded at the end of an assignment loop or singular assignment run.
type assignmentClient struct {
	log        *logrus.Entry
	oktaClient api.Client

	// Mapping of usernames to user IDs. Initialized once on first read and then
	// only ever read from, so locking isn't an issue.
	users map[userName]oktaUserID

	// Guard for initializing the `users` map
	initUsersOnce sync.Once

	// Group membership.
	groups utils.SyncMap[oktaGroupID, set.Set[oktaUserID]]

	// Apps membership.
	apps utils.SyncMap[oktaAppID, set.Set[api.AppAssignment]]

	// syncSingleFlight is used to prevent multiple requests during listing user apps groups
	// assignments. Parallel calls will be collapsed in to one and all receive the same result.
	// Assumed that Okta GroupID and AppID  are exclusive uniq (Okta Group ID != Okta App ID)
	syncSingleFlight singleflight.Group
}

// newAssignmentClient will return a new assignment client.
func newAssignmentClient(log *logrus.Entry, oktaClient api.Client) *assignmentClient {
	return &assignmentClient{
		log:        log,
		oktaClient: oktaClient,
	}
}

func (a *assignmentClient) getGroupAssignments(ctx context.Context, groupID oktaGroupID) (set.Set[oktaUserID], error) {
	// syncSingleFlight is used to prevent multiple requests during listing user groups assignments.
	// assignments are processed in parallel (See processAssignments function)
	// We need to make sure that we don't make multiple requests to Okta for the same group.
	// After the first request, the result is cached and returned for subsequent requests.
	key := fmt.Sprintf("group:%s", groupID)
	items, err, _ := a.syncSingleFlight.Do(key, func() (interface{}, error) {
		cachedMembers, populated := a.groups.Load(groupID)
		if populated {
			return cachedMembers, nil
		}

		a.log.Debugf("Refreshing assignments for group %s", groupID)
		members, err := a.oktaClient.GetGroupAssignments(ctx, groupID)
		if err != nil {
			return false, trace.Wrap(err)
		}
		cachedMembers = set.New[oktaUserID](members...)
		a.log.
			WithFields(logrus.Fields{
				"members": members,
				"groupID": groupID,
			}).
			Debugf("Found users assigned to group")
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
		ok = members.Has(userID)
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
		a.log.Debugf("User %s is already assigned to group %s", userID, groupID)
		return nil
	}

	if err := a.oktaClient.AssignUserToGroup(ctx, userID, groupID); err != nil {
		return trace.Wrap(err)
	}

	a.log.Debugf("User %s has been assigned to group %s", userID, groupID)

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

	log := a.log.WithFields(logrus.Fields{"user": userID, "group": groupID})

	// Already unregistered.
	if !ok {
		log.Debug("User is already unassigned from group")
		return nil
	}

	if err := a.oktaClient.UnassignUserFromGroup(ctx, userID, groupID); err != nil {
		if oErr := (*api.OktaAPIValidationError)(nil); errors.As(err, &oErr) {
			// This is referring to Okta group rules:
			// https://help.okta.com/en-us/Content/Topics/users-groups-profiles/usgp-about-group-rules.htm
			log.Warn("Unable to remove user from group due to API validation exception. This membership is likely managed by Okta group rules. Proceeding as if this were successful.")
		} else if trace.IsNotFound(err) {
			log.Warn("Unable to remove user from group due because the group cannot be found in Okta. Proceeding as if this were successful.")
		} else {
			return trace.Wrap(err)
		}
	} else {
		log.Debug("User has been unassigned from groups")
	}

	// Update the local group cache to match the upstream Okta organization
	a.groups.Write(func(groups map[oktaGroupID]set.Set[oktaUserID]) {
		groups[groupID].Remove(userID)
	})

	return nil
}

func (a *assignmentClient) getUserAssignedToApp(ctx context.Context, appID oktaAppID) (set.Set[api.AppAssignment], error) {
	// syncSingleFlight is used to prevent multiple requests during listing user apps assignments.
	// assignments are processed in parallel (See processAssignments function)
	// We need to make sure that we don't make multiple requests to Okta for the same app.
	// After the first request, the result is cached and returned for subsequent requests.
	key := fmt.Sprintf("app:%s", appID)
	items, err, _ := a.syncSingleFlight.Do(key, func() (interface{}, error) {
		cached, populated := a.apps.Load(appID)
		if populated {
			return cached, nil
		}
		a.log.Debugf("Refreshing assignments for app %s", appID)
		items, err := a.oktaClient.GetAppAssignments(ctx, appID)
		if err != nil {
			return false, trace.Wrap(err)
		}
		cached = set.New[api.AppAssignment]()
		for _, member := range items {
			cached.Add(member)
		}
		a.log.
			WithFields(logrus.Fields{
				"members": maps.Keys(cached),
				"appID":   appID,
			}).
			Debugf("Found users assigned to app")

		a.apps.Store(appID, cached)
		return cached, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members, ok := items.(set.Set[api.AppAssignment])
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
	a.apps.Read(func(_ map[oktaAppID]set.Set[api.AppAssignment]) {
		assignedToGroup = assignments.Has(api.AppAssignment{UserID: string(userID), Scope: api.GroupScope})
		assignedToApp = assignments.Has(api.AppAssignment{UserID: string(userID), Scope: api.UserScope})
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
		a.log.Debugf("User %s is already assigned to app %s", userID, appID)
		return nil
	}

	// Make update to upstream Okta
	if err := a.oktaClient.AssignUserToApplication(ctx, userID, appID); err != nil {
		return trace.Wrap(err)
	}

	// Update local cache
	a.log.Debugf("User %s has been assigned to app %s", userID, appID)
	a.apps.Write(func(apps map[oktaAppID]set.Set[api.AppAssignment]) {
		apps[appID].Add(api.AppAssignment{UserID: string(userID), Scope: api.UserScope})
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

	log := a.log.WithFields(logrus.Fields{"user": userID, "application": appID})

	// Already unregistered.
	if !ok {
		log.Debug("User has already been unassigned from app")
		return nil
	}

	if err := a.oktaClient.UnassignUserFromApplication(ctx, userID, appID); err != nil {
		if oErr := (*api.OktaAPIValidationError)(nil); errors.As(err, &oErr) {
			// This is referring to Okta group rules:
			// https://help.okta.com/en-us/Content/Topics/users-groups-profiles/usgp-about-group-rules.htm
			log.Warn("Unable to remove user from application due to API validation exception. Proceeding as if this were successful.")
		} else if trace.IsNotFound(err) {
			log.Warn("Unable to remove user from application due because the application cannot be found in Okta. Proceeding as if this were successful.")
		} else {
			return trace.Wrap(err)
		}
	} else {
		log.Debug("User has been unassigned from app")
	}

	// Update local cache
	a.apps.Write(func(apps map[oktaAppID]set.Set[api.AppAssignment]) {
		apps[appID].Remove(api.AppAssignment{UserID: string(userID), Scope: api.UserScope})
	})

	return nil
}

// userID will return the Okta userID for the username.
func (a *assignmentClient) userID(ctx context.Context, username userName) (oktaUserID, error) {
	var err error

	a.initUsersOnce.Do(func() {
		a.log.Debugf("Refreshing organization user list")
		a.users, err = a.oktaClient.ListUsers(ctx)
		if err != nil {
			err = trace.Wrap(err)
			return
		}

		// Assignment can be stale and refer to deactivated user.
		// userID function needs succeed to successfully process and clean the assignment.
		// We need to make sure that all assignments for deactivated user was cleanup
		// before deleting okta assigment.
		filter := fmt.Sprintf(`status eq "%s"`, userStatusDeprovisioned)
		var deactivatedUsers map[userName]oktaUserID
		deactivatedUsers, err = a.oktaClient.ListUsers(ctx, query.WithFilter(filter))
		if err != nil {
			err = trace.Wrap(err)
			return
		}

		if a.users == nil {
			a.users = make(map[api.UserName]api.OktaUserID)
		}
		maps.Copy(a.users, deactivatedUsers)
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	uid, ok := a.users[username]
	if !ok {
		return "", trace.NotFound("unable to find ID for user %s", username)
	}

	return uid, nil
}
