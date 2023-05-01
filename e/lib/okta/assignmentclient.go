/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"
)

const (
	assignmentClientOktaTimeout = 10 * time.Second
)

// assignmentClient is a caching Okta client that will keep track of of Okta
// state while assignments are processed.  When querying for assignment state
// for a group or app, if the current list of assignments to the target has not
// yet been retrieved from Okta, it will be retrieved upon request. This client
// should be discarded at the end of an assignment loop or singular assignment run.
type assignmentClient struct {
	oktaClient oktaClient

	rateLimiter *rate.Limiter

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
func newAssignmentClient(oktaClient oktaClient, rateLimiter *rate.Limiter) *assignmentClient {
	return &assignmentClient{
		oktaClient:  oktaClient,
		rateLimiter: rateLimiter,
		groups:      map[string]map[string]bool{},
		apps:        map[string]map[string]bool{},
	}
}

// userAssignedToGroup will return true if the user is assigned to the group.
func (a *assignmentClient) userAssignedToGroup(ctx context.Context, username, groupID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	userID, err := a.userID(ctx, username)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the group entry hasn't yet been populated, populate it.
	a.groupsMu.Lock()
	assignments := a.groups[groupID]
	a.groupsMu.Unlock()

	if assignments == nil {
		if err := a.rateLimiter.Wait(ctx); err != nil {
			return false, trace.Wrap(err)
		}

		members, err := a.oktaClient.getGroupAssignments(ctx, groupID)
		if err != nil {
			return false, trace.Wrap(err)
		}

		a.groupsMu.Lock()
		a.groups[groupID] = map[string]bool{}
		for _, member := range members {
			a.groups[groupID][member] = true
		}
		a.groupsMu.Unlock()
	}

	a.groupsMu.RLock()
	_, ok := a.groups[groupID][userID]
	a.groupsMu.RUnlock()

	return ok, nil
}

// registerUserToGroup will register the user to the group.
func (a *assignmentClient) registerUserToGroup(ctx context.Context, username, groupID string) error {
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	ok, err := a.userAssignedToGroup(ctx, username, groupID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already registered.
	if ok {
		return nil
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.oktaClient.assignUserToGroup(ctx, userID, groupID); err != nil {
		return trace.Wrap(err)
	}

	a.groupsMu.Lock()
	a.groups[groupID][userID] = true
	a.groupsMu.Unlock()

	return nil
}

// unregisterUserFromGroup will unregister the user from the group.
func (a *assignmentClient) unregisterUserFromGroup(ctx context.Context, username, groupID string) error {
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	ok, err := a.userAssignedToGroup(ctx, username, groupID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already unregistered.
	if !ok {
		return nil
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := a.oktaClient.unassignUserFromGroup(ctx, userID, groupID); err != nil {
		return trace.Wrap(err)
	}

	a.groupsMu.Lock()
	delete(a.groups[groupID], userID)
	a.groupsMu.Unlock()

	return nil
}

// userAssignedToApp will return true if the user is assigned to the app.
func (a *assignmentClient) userAssignedToApp(ctx context.Context, username, appID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	userID, err := a.userID(ctx, username)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the app entry hasn't yet been populated, populate it.
	a.appsMu.RLock()
	assignments := a.apps[appID]
	a.appsMu.RUnlock()

	if assignments == nil {
		if err := a.rateLimiter.Wait(ctx); err != nil {
			return false, trace.Wrap(err)
		}

		members, err := a.oktaClient.getAppAssignments(ctx, appID)
		if err != nil {
			return false, trace.Wrap(err)
		}

		a.appsMu.Lock()
		a.apps[appID] = map[string]bool{}
		for _, member := range members {
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
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	ok, err := a.userAssignedToApp(ctx, username, appID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already registered.
	if ok {
		return nil
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := a.oktaClient.assignUserToApplication(ctx, userID, appID); err != nil {
		return trace.Wrap(err)
	}

	a.appsMu.Lock()
	a.apps[appID][userID] = true
	a.appsMu.Unlock()

	return nil
}

// unregisterUserFromGroup will unregister the user from the app.
func (a *assignmentClient) unregisterUserFromApp(ctx context.Context, username, appID string) error {
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	ok, err := a.userAssignedToApp(ctx, username, appID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Already unregistered.
	if !ok {
		return nil
	}

	userID, err := a.userID(ctx, username)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.rateLimiter.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	if err := a.oktaClient.unassignUserFromApplication(ctx, userID, appID); err != nil {
		return trace.Wrap(err)
	}

	a.appsMu.Lock()
	delete(a.apps[appID], userID)
	a.appsMu.Unlock()

	return nil
}

// userID will return the userID for the username.
func (a *assignmentClient) userID(ctx context.Context, username string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, assignmentClientOktaTimeout)
	defer cancel()

	a.usersMu.Lock()
	if a.users == nil {
		var err error

		if err := a.rateLimiter.Wait(ctx); err != nil {
			return "", trace.Wrap(err)
		}

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
