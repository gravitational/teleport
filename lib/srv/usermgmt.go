/*
Copyright 2022 Gravitational, Inc.

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

package srv

import (
	"context"
	"errors"
	"io"
	"os/user"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

// NewHostUsers initialize a new HostUsers object
func NewHostUsers(ctx context.Context, storage *local.PresenceService, uuid string) HostUsers {
	// newHostUsersBackend statically returns a valid backend or an error,
	// resulting in a staticcheck linter error on darwin
	backend, err := newHostUsersBackend(uuid) //nolint:staticcheck
	if err != nil {                           //nolint:staticcheck
		log.Warnf("Error making new HostUsersBackend: %s", err)
		return nil
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	return &HostUserManagement{
		backend:   backend,
		ctx:       cancelCtx,
		cancel:    cancelFunc,
		storage:   storage,
		userGrace: time.Second * 30,
	}
}

type HostUsersBackend interface {
	// GetAllUsers returns all host users on a node.
	GetAllUsers() ([]string, error)
	// UserGIDs returns a list of group ids for a user.
	UserGIDs(*user.User) ([]string, error)
	// Lookup retrieves a user by name.
	Lookup(name string) (*user.User, error)
	// LookupGroup retrieves a group by name.
	LookupGroup(group string) (*user.Group, error)
	// CreateGroup creates a group on a host.
	CreateGroup(group string) error
	// CreateUser creates a user on a host.
	CreateUser(name string, groups []string) error
	// DeleteUser deletes a user from a host.
	DeleteUser(name string) error
	// CheckSudoers ensures that a sudoers file to be written is valid
	CheckSudoers(contents []byte) error
	// WriteSudoersFile creates the user's sudoers file.
	WriteSudoersFile(user string, entries []byte) error
	// RemoveSudoersFile deletes a user's sudoers file.
	RemoveSudoersFile(user string) error
}

// HostUsersProvisioningBackend is used to implement HostUsersBackend
type HostUsersProvisioningBackend struct {
	sudoersPath string
	hostUUID    string
}

type userCloser struct {
	users    HostUsers
	backend  HostUsersBackend
	username string
}

func (u *userCloser) Close() error {
	teleportGroup, err := u.backend.LookupGroup(types.TeleportServiceGroup)
	if err != nil {
		return trace.Wrap(err)
	}
	err = u.users.doWithUserLock(func(sl types.SemaphoreLease) error {
		return trace.Wrap(u.users.DeleteUser(u.username, teleportGroup.Gid))
	})
	return trace.Wrap(err)
}

var ErrUserLoggedIn = errors.New("User logged in error")

type HostUsers interface {
	// CreateUser creates a temporary Teleport user in the TeleportServiceGroup
	CreateUser(name string, hostRoleInfo *services.HostUsersInfo) (*user.User, io.Closer, error)
	// DeleteUser deletes a temporary Teleport user only if they are
	// in a specified group
	DeleteUser(name string, gid string) error
	// DeleteAllUsers deletes all suer in the TeleportServiceGroup
	DeleteAllUsers() error
	// UserCleanup starts a periodic user deletion cleanup loop for
	// users that failed to delete
	UserCleanup()
	// Shutdown cancels the UserCleanup loop
	Shutdown()

	// UserExists returns nil should a hostuser exist
	UserExists(string) (*user.User, error)

	// doWithUserLock runs the passed function with a host user
	// interaction lock
	doWithUserLock(func(types.SemaphoreLease) error) error

	// SetHostUserDeletionGrace sets the grace period before a user
	// can be deleted, used so integration tests dont need to sleep
	SetHostUserDeletionGrace(time.Duration)
}

type HostUserManagement struct {
	backend HostUsersBackend
	ctx     context.Context
	cancel  context.CancelFunc
	storage *local.PresenceService

	userGrace time.Duration
}

var _ HostUsers = &HostUserManagement{}

// CreateUser creates a temporary Teleport user in the TeleportServiceGroup
func (u *HostUserManagement) CreateUser(name string, ui *services.HostUsersInfo) (*user.User, io.Closer, error) {
	tempUser, err := u.backend.Lookup(name)
	if err != nil && err != user.UnknownUserError(name) {
		return nil, nil, trace.Wrap(err)
	}

	if tempUser != nil {
		gids, err := u.backend.UserGIDs(tempUser)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		systemGroup, err := u.backend.LookupGroup(types.TeleportServiceGroup)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		var found bool
		for _, gid := range gids {
			if gid == systemGroup.Gid {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, trace.AlreadyExists("User %q already exists and is not managed by teleport", name)
		}

		err = u.doWithUserLock(func(_ types.SemaphoreLease) error {
			if err := u.storage.UpsertHostUserInteractionTime(u.ctx, name, time.Now()); err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		// try to delete even if the user already exists as only users
		// in the teleport-system group will be deleted and this way
		// if a user creates multiple sessions the account will
		// succeed in deletion
		return tempUser, &userCloser{
			username: name,
			users:    u,
			backend:  u.backend,
		}, trace.AlreadyExists("User %q already exists", name)
	}

	groups := append(ui.Groups, types.TeleportServiceGroup)
	var errs []error
	for _, group := range groups {
		if err := u.createGroupIfNotExist(group); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	if err := trace.NewAggregate(errs...); err != nil {
		return nil, nil, trace.WrapWithMessage(err, "error while creating groups")
	}

	err = u.doWithUserLock(func(_ types.SemaphoreLease) error {
		if err := u.storage.UpsertHostUserInteractionTime(u.ctx, name, time.Now()); err != nil {
			return trace.Wrap(err)
		}

		err = u.backend.CreateUser(name, groups)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.WrapWithMessage(err, "error while creating user")
		}
		return nil
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tempUser, err = u.backend.Lookup(name)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	closer := &userCloser{
		username: name,
		users:    u,
		backend:  u.backend,
	}
	if len(ui.Sudoers) != 0 {
		contents := []byte(strings.Join(ui.Sudoers, "\n") + "\n")
		err := u.backend.WriteSudoersFile(name, contents)
		if err != nil {
			return tempUser, closer, trace.Wrap(err)
		}
	}

	return tempUser, closer, nil
}

func (u *HostUserManagement) doWithUserLock(f func(types.SemaphoreLease) error) error {
	lock, err := services.AcquireSemaphoreWithRetry(u.ctx,
		services.AcquireSemaphoreWithRetryConfig{
			Service: u.storage,
			Request: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.SemaphoreKindHostUserModification,
				SemaphoreName: "host_user_modification",
				MaxLeases:     1,
				Expires:       time.Now().Add(time.Second * 20),
			},
			Retry: utils.LinearConfig{
				Step: time.Second * 5,
				Max:  time.Minute,
			},
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer u.storage.CancelSemaphoreLease(u.ctx, *lock)
	return trace.Wrap(f(*lock))
}

func (u *HostUserManagement) createGroupIfNotExist(group string) error {
	_, err := u.backend.LookupGroup(group)
	if err != nil && err != user.UnknownGroupError(group) {
		return trace.Wrap(err)
	}
	err = u.backend.CreateGroup(group)
	if trace.IsAlreadyExists(err) {
		return nil
	}
	return trace.Wrap(err)
}

// DeleteAllUsers deletes all host users in the teleport service group.
func (u *HostUserManagement) DeleteAllUsers() error {
	users, err := u.backend.GetAllUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	teleportGroup, err := u.backend.LookupGroup(types.TeleportServiceGroup)
	if err != nil {
		if errors.Is(err, user.UnknownGroupError(types.TeleportServiceGroup)) {
			log.Debugf("'teleport-service' group not found, not deleting users")
			return nil
		}
		return trace.Wrap(err)
	}
	var errs []error
	for _, name := range users {
		lt, err := u.storage.GetHostUserInteractionTime(u.ctx, name)
		if err != nil {
			log.Debugf("Failed to find user %q login time: %s", name, err)
			continue
		}
		u.doWithUserLock(func(l types.SemaphoreLease) error {
			if time.Since(lt) < u.userGrace {
				// small grace period in order to avoid deleting users
				// in-between them starting their SSH session and
				// entering the shell
				return nil
			}
			errs = append(errs, u.DeleteUser(name, teleportGroup.Gid))

			l.Expires = time.Now().Add(time.Second * 10)
			u.storage.KeepAliveSemaphoreLease(u.ctx, l)
			return nil
		})
	}
	return trace.NewAggregate(errs...)
}

// DeleteUser deletes the user only if they are
// present in the specified group
func (u *HostUserManagement) DeleteUser(username string, gid string) error {
	tempUser, err := u.backend.Lookup(username)
	if err != nil {
		return trace.Wrap(err)
	}
	ids, err := u.backend.UserGIDs(tempUser)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, id := range ids {
		if id == gid {
			err := u.backend.DeleteUser(username)
			if err != nil {
				if errors.Is(err, ErrUserLoggedIn) {
					log.Debugf("Not deleting user %q, user has another session, or running process", username)
					return nil
				}
				return trace.Wrap(err)
			}

			if err := u.backend.RemoveSudoersFile(username); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
	}
	log.Debugf("User %q not deleted: not a temporary user", username)
	return nil
}

// UserCleanup starts a periodic user deletion cleanup loop for
// users that failed to delete
func (u *HostUserManagement) UserCleanup() {
	cleanupTicker := time.NewTicker(time.Minute * 5)
	defer cleanupTicker.Stop()
	for {
		if err := u.DeleteAllUsers(); err != nil {
			log.Error("Error during temporary user cleanup: ", err)
		}
		select {
		case <-cleanupTicker.C:
		case <-u.ctx.Done():
			return
		}
	}
}

// Shutdown cancels the UserCleanup loop
func (u *HostUserManagement) Shutdown() {
	u.cancel()
}

// UserExists returns nil should a hostuser exist
func (u *HostUserManagement) UserExists(username string) (*user.User, error) {
	tempUser, err := u.backend.Lookup(username)
	if err != nil {
		if err == user.UnknownUserError(username) {
			return nil, trace.NotFound("User not found: %s", err)
		}
		return nil, trace.Wrap(err)
	}
	return tempUser, nil
}

func (u *HostUserManagement) SetHostUserDeletionGrace(d time.Duration) {
	u.userGrace = d
}
