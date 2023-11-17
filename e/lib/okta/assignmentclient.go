package okta

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// assignmentClient is a caching Okta client that will keep track of of Okta
// state while assignments are processed.  When querying for assignment state
// for a group or app, if the current list of assignments to the target has not
// yet been retrieved from Okta, it will be retrieved upon request. This client
// should be discarded at the end of an assignment loop or singular assignment run.
type assignmentClient struct {
	log        *logrus.Entry
	oktaClient oktaClient

	// Mapping of usernames to user IDs.
	usersMu sync.Mutex
	users   map[string]string

	// Group membership.
	groupsMu sync.RWMutex
	groups   map[string]map[string]bool

	// Apps membership.
	appsMu sync.RWMutex
	apps   map[string]map[string]bool
}

// newAssignmentClient will return a new assignment client.
func newAssignmentClient(log *logrus.Entry, oktaClient oktaClient) *assignmentClient {
	return &assignmentClient{
		log:        log,
		oktaClient: oktaClient,
		groups:     map[string]map[string]bool{},
		apps:       map[string]map[string]bool{},
	}
}

// userAssignedToGroup will return true if the user is assigned to the group.
func (a *assignmentClient) userAssignedToGroup(ctx context.Context, username, groupID string) (bool, error) {
	userID, err := a.userID(ctx, username)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the group entry hasn't yet been populated, populate it.
	a.groupsMu.Lock()
	assignments := a.groups[groupID]
	a.groupsMu.Unlock()

	if assignments == nil {
		a.log.Debugf("Refreshing assignments for group %s", groupID)
		members, err := a.oktaClient.getGroupAssignments(ctx, groupID)
		if err != nil {
			return false, trace.Wrap(err)
		}

		a.groupsMu.Lock()
		a.groups[groupID] = map[string]bool{}
		for _, member := range members {
			a.log.Debugf("Found user %s assigned to group %s", member, groupID)
			a.groups[groupID][member] = true
		}
		a.groupsMu.Unlock()
	}

	a.groupsMu.RLock()
	ok := a.groups[groupID][userID]
	a.groupsMu.RUnlock()

	return ok, nil
}

// registerUserToGroup will register the user to the group.
func (a *assignmentClient) registerUserToGroup(ctx context.Context, username, groupID string) error {
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

	if err := a.oktaClient.assignUserToGroup(ctx, userID, groupID); err != nil {
		return trace.Wrap(err)
	}

	a.log.Debugf("User %s has been assigned to group %s", userID, groupID)

	a.groupsMu.Lock()
	a.groups[groupID][userID] = true
	a.groupsMu.Unlock()

	return nil
}

// unregisterUserFromGroup will unregister the user from the group.
func (a *assignmentClient) unregisterUserFromGroup(ctx context.Context, username, groupID string) error {
	ok, err := a.userAssignedToGroup(ctx, username, groupID)
	if err != nil {
		return trace.Wrap(err)
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already unregistered.
	if !ok {
		a.log.Debugf("User %s is already unassigned from group %s", userID, groupID)
		return nil
	}

	if err := a.oktaClient.unassignUserFromGroup(ctx, userID, groupID); err != nil {
		if _, ok := err.(oktaAPIValidationError); !ok {
			return trace.Wrap(err)
		}

		// This is referring to Okta group rules:
		// https://help.okta.com/en-us/Content/Topics/users-groups-profiles/usgp-about-group-rules.htm
		a.log.Warnf("Unable to remove user %s from group %s due to API validation exception. This membership is likely managed by Okta group rules. Proceeding as if this were successful.", userID, groupID)
	} else {
		a.log.Debugf("User %s has been unassigned to group %s", userID, groupID)
	}

	a.groupsMu.Lock()
	delete(a.groups[groupID], userID)
	a.groupsMu.Unlock()

	return nil
}

// userAssignedToApp will return true if the user is assigned to the app.
func (a *assignmentClient) userAssignedToApp(ctx context.Context, username, appID string) (bool, error) {
	userID, err := a.userID(ctx, username)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the app entry hasn't yet been populated, populate it.
	a.appsMu.RLock()
	assignments := a.apps[appID]
	a.appsMu.RUnlock()

	if assignments == nil {
		a.log.Debugf("Refreshing assignments for app %s", appID)
		members, err := a.oktaClient.getAppAssignments(ctx, appID)
		if err != nil {
			return false, trace.Wrap(err)
		}

		a.appsMu.Lock()
		a.apps[appID] = map[string]bool{}
		for _, member := range members {
			a.log.Debugf("Found user %s assigned to app %s", member, appID)
			a.apps[appID][member] = true
		}
		a.appsMu.Unlock()
	}

	a.appsMu.RLock()
	_, ok := a.apps[appID][userID]
	a.appsMu.RUnlock()

	return ok, nil
}

// registerUserToApp will register the user to the app.
func (a *assignmentClient) registerUserToApp(ctx context.Context, username, appID string) error {
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

	if err := a.oktaClient.assignUserToApplication(ctx, userID, appID); err != nil {
		return trace.Wrap(err)
	}

	a.log.Debugf("User %s has been assigned to app %s", userID, appID)

	a.appsMu.Lock()
	a.apps[appID][userID] = true
	a.appsMu.Unlock()

	return nil
}

// unregisterUserFromGroup will unregister the user from the app.
func (a *assignmentClient) unregisterUserFromApp(ctx context.Context, username, appID string) error {
	ok, err := a.userAssignedToApp(ctx, username, appID)
	if err != nil {
		return trace.Wrap(err)
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already unregistered.
	if !ok {
		a.log.Debugf("User %s has already been unassigned from app %s", userID, appID)
		return nil
	}

	if err := a.oktaClient.unassignUserFromApplication(ctx, userID, appID); err != nil {
		if _, ok := err.(oktaAPIValidationError); !ok {
			return trace.Wrap(err)
		}
		a.log.Warnf("Unable to remove user %s from application %s due to API validation exception. Proceeding as if this were successful.", userID, appID)
	} else {
		a.log.Debugf("User %s has been unassigned from app %s", userID, appID)
	}

	a.appsMu.Lock()
	delete(a.apps[appID], userID)
	a.appsMu.Unlock()

	return nil
}

// userID will return the userID for the username.
func (a *assignmentClient) userID(ctx context.Context, username string) (string, error) {
	a.usersMu.Lock()
	if a.users == nil {
		var err error

		a.users, err = a.oktaClient.listUsers(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}
	a.usersMu.Unlock()

	user, ok := a.users[username]

	if ok {
		return user, nil
	}

	return "", trace.NotFound("unable to find ID for user %s", username)
}
