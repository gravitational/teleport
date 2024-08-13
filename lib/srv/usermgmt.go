/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/user"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

// NewHostUsers initialize a new HostUsers object
func NewHostUsers(ctx context.Context, storage services.PresenceInternal, uuid string) HostUsers {
	//nolint:staticcheck // SA4023. False positive on macOS.
	backend, err := newHostUsersBackend()
	switch {
	case trace.IsNotImplemented(err), trace.IsNotFound(err):
		log.Debugf("Skipping host user management: %v", err)
		return nil
	case err != nil: //nolint:staticcheck // linter fails on non-linux system as only linux implementation returns useful values.
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

func NewHostSudoers(uuid string) HostSudoers {
	//nolint:staticcheck // SA4023. False positive on macOS.
	backend, err := newHostSudoersBackend(uuid)
	switch {
	case trace.IsNotImplemented(err):
		log.Debugf("Skipping host sudoers management: %v", err)
		return nil
	case err != nil: //nolint:staticcheck // linter fails on non-linux system as only linux implementation returns useful values.
		log.Warnf("Error making new HostSudoersBackend: %s", err)
		return nil
	}
	return &HostSudoersManagement{
		backend: backend,
	}
}

type HostSudoersBackend interface {
	// CheckSudoers ensures that a sudoers file to be written is valid
	CheckSudoers(contents []byte) error
	// WriteSudoersFile creates the user's sudoers file.
	WriteSudoersFile(user string, entries []byte) error
	// RemoveSudoersFile deletes a user's sudoers file.
	RemoveSudoersFile(user string) error
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
	// LookupGroupByID retrieves a group by its ID.
	LookupGroupByID(gid string) (*user.Group, error)
	// SetUserGroups sets a user's groups, replacing their existing groups.
	SetUserGroups(name string, groups []string) error
	// CreateGroup creates a group on a host.
	CreateGroup(group string, gid string) error
	// CreateUser creates a user on a host.
	CreateUser(name string, groups []string, home, uid, gid string) error
	// DeleteUser deletes a user from a host.
	DeleteUser(name string) error
	// CreateHomeDirectory creates the users home directory and copies in /etc/skel
	CreateHomeDirectory(userHome string, uid, gid string) error
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

type HostSudoers interface {
	// WriteSudoers creates a temporary Teleport user in the TeleportServiceGroup
	WriteSudoers(name string, sudoers []string) error
	// RemoveSudoers removes the users sudoer file
	RemoveSudoers(name string) error
}

type HostSudoersNotImplemented struct{}

// WriteSudoers creates a temporary Teleport user in the TeleportServiceGroup
func (*HostSudoersNotImplemented) WriteSudoers(string, []string) error {
	return trace.NotImplemented("host sudoers functionality not implemented on this platform")
}

// RemoveSudoers removes the users sudoer file
func (*HostSudoersNotImplemented) RemoveSudoers(name string) error {
	return trace.NotImplemented("host sudoers functionality not implemented on this platform")
}

type HostUsers interface {
	// UpsertUser creates a temporary Teleport user in the TeleportServiceGroup
	UpsertUser(name string, hostRoleInfo *services.HostUsersInfo) (io.Closer, error)
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
	UserExists(string) error

	// doWithUserLock runs the passed function with a host user
	// interaction lock
	doWithUserLock(func(types.SemaphoreLease) error) error

	// SetHostUserDeletionGrace sets the grace period before a user
	// can be deleted, used so integration tests don't need to sleep
	SetHostUserDeletionGrace(time.Duration)
}

type HostUserManagement struct {
	backend HostUsersBackend
	ctx     context.Context
	cancel  context.CancelFunc
	storage services.PresenceInternal

	userGrace time.Duration
}

type HostSudoersManagement struct {
	backend HostSudoersBackend
}

var (
	_ HostUsers   = &HostUserManagement{}
	_ HostSudoers = &HostSudoersManagement{}
)

// Under the section "Including other files from within sudoers":
//
//	https://man7.org/linux/man-pages/man5/sudoers.5.html
//
// '.', '~' and '/' will cause a file not to be read and these can be
// included in a username, removing slash to avoid escaping a
// directory
var sudoersSanitizationMatcher = regexp.MustCompile(`[\.~\/]`)

// sanitizeSudoersName replaces occurrences of '.', '~' and '/' with
// underscores as `sudo` will not read files including these
// characters
func sanitizeSudoersName(username string) string {
	return sudoersSanitizationMatcher.ReplaceAllString(username, "_")
}

// WriteSudoers creates a sudoers file for a user from a list of entries
func (u *HostSudoersManagement) WriteSudoers(name string, sudoers []string) error {
	var sudoersOut strings.Builder
	for _, entry := range sudoers {
		sudoersOut.WriteString(fmt.Sprintf("%s %s\n", name, entry))
	}
	err := u.backend.WriteSudoersFile(name, []byte(sudoersOut.String()))
	return trace.Wrap(err)
}

func (u *HostSudoersManagement) RemoveSudoers(name string) error {
	if err := u.backend.RemoveSudoersFile(name); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertUser creates a temporary Teleport user in the TeleportServiceGroup
func (u *HostUserManagement) UpsertUser(name string, ui *services.HostUsersInfo) (io.Closer, error) {
	if ui.Mode == types.CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED {
		return nil, trace.BadParameter("Mode is a required argument to CreateUser")
	}

	groupsToAdd := make([]string, 0, len(ui.Groups))
	for _, group := range ui.Groups {
		if group == name {
			// this causes an error as useradd expects the group with the same name as the user to be available
			log.Debugf("Skipping group creation with name the same as login user (%q, %q).", name, group)
			continue
		}
		groupsToAdd = append(groupsToAdd, group)
	}
	if ui.Mode == types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP {
		groupsToAdd = append(groupsToAdd, types.TeleportServiceGroup)
	}
	var errs []error
	for _, group := range groupsToAdd {
		if err := u.createGroupIfNotExist(group); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	if err := trace.NewAggregate(errs...); err != nil {
		return nil, trace.WrapWithMessage(err, "error while creating groups")
	}

	tempUser, err := u.backend.Lookup(name)
	if err != nil && !errors.Is(err, user.UnknownUserError(name)) {
		return nil, trace.Wrap(err)
	}

	if tempUser != nil {
		// Collect actions that need to be done together under a lock on the user.
		actionsUnderLock := make([]func() error, 0, 2)
		doWithUserLock := func() error {
			if len(actionsUnderLock) == 0 {
				return nil
			}

			return trace.Wrap(u.doWithUserLock(func(_ types.SemaphoreLease) error {
				for _, action := range actionsUnderLock {
					if err := action(); err != nil {
						return trace.Wrap(err)
					}
				}
				return nil
			}))
		}

		// Get the user's current groups.
		currentGroups := make(map[string]struct{}, len(groupsToAdd))
		groupIds, err := u.backend.UserGIDs(tempUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, groupId := range groupIds {
			group, err := u.backend.LookupGroupByID(groupId)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			currentGroups[group.Name] = struct{}{}
		}

		// Get the groups that the user should end up with, including the primary group.
		finalGroups := make(map[string]struct{}, len(groupsToAdd)+1)
		for _, group := range groupsToAdd {
			finalGroups[group] = struct{}{}
		}
		primaryGroup, err := u.backend.LookupGroupByID(tempUser.Gid)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		finalGroups[primaryGroup.Name] = struct{}{}

		// Check if the user's groups need to be updated.
		if !maps.Equal(currentGroups, finalGroups) {
			actionsUnderLock = append(actionsUnderLock, func() error {
				return trace.Wrap(u.backend.SetUserGroups(name, groupsToAdd))
			})
		}

		systemGroup, err := u.backend.LookupGroup(types.TeleportServiceGroup)
		if err != nil {
			if isUnknownGroupError(err, types.TeleportServiceGroup) {
				// Teleport service group doesn't exist, so we don't need to update interaction time.
				return nil, trace.Wrap(doWithUserLock())
			}
			return nil, trace.Wrap(err)
		}
		gids, err := u.backend.UserGIDs(tempUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var found bool
		for _, gid := range gids {
			if gid == systemGroup.Gid {
				found = true
				break
			}
		}
		if !found {
			// User isn't managed by Teleport, so we don't need to update interaction time.
			return nil, trace.Wrap(doWithUserLock())
		}

		actionsUnderLock = append(actionsUnderLock, func() error {
			return trace.Wrap(u.storage.UpsertHostUserInteractionTime(u.ctx, name, time.Now()))
		})
		if err := doWithUserLock(); err != nil {
			return nil, trace.Wrap(err)
		}
		// try to delete even if the user already exists as only users
		// in the teleport-system group will be deleted and this way
		// if a user creates multiple sessions the account will
		// succeed in deletion
		return &userCloser{
			username: name,
			users:    u,
			backend:  u.backend,
		}, nil
	}

	var home string
	if ui.Mode != types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP {
		//nolint:staticcheck // SA4023. False positive on macOS.
		home, err = readDefaultHome(name)
		//nolint:staticcheck // SA4023. False positive on macOS.
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	err = u.doWithUserLock(func(_ types.SemaphoreLease) error {
		if ui.Mode != types.CreateHostUserMode_HOST_USER_MODE_KEEP {
			if err := u.storage.UpsertHostUserInteractionTime(u.ctx, name, time.Now()); err != nil {
				return trace.Wrap(err)
			}
		}
		if ui.GID != "" {
			// if gid is specified a group must already exist
			err := u.backend.CreateGroup(name, ui.GID)
			if err != nil && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}

		err = u.backend.CreateUser(name, groupsToAdd, home, ui.UID, ui.GID)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.WrapWithMessage(err, "error while creating user")
		}

		user, err := u.backend.Lookup(name)
		if err != nil {
			return trace.Wrap(err)
		}

		if home != "" {
			if err := u.backend.CreateHomeDirectory(home, user.Uid, user.Gid); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ui.Mode == types.CreateHostUserMode_HOST_USER_MODE_KEEP {
		return nil, nil
	}

	closer := &userCloser{
		username: name,
		users:    u,
		backend:  u.backend,
	}

	return closer, trace.Wrap(err)
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
			Retry: retryutils.LinearConfig{
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
	if err != nil && !isUnknownGroupError(err, group) {
		return trace.Wrap(err)
	}
	err = u.backend.CreateGroup(group, "")
	if trace.IsAlreadyExists(err) {
		return nil
	}
	return trace.Wrap(err)
}

// isUnknownGroupError returns whether the error from LookupGroup is an unknown group error.
//
// LookupGroup is supposed to return an UnknownGroupError, but due to an existing issue
// may instead return a generic "no such file or directory" error when sssd is installed
// or "no such process" as Go std library just forwards errors returned by getgrpnam_r.
// See github issue - https://github.com/golang/go/issues/40334
func isUnknownGroupError(err error, groupName string) bool {
	return errors.Is(err, user.UnknownGroupError(groupName)) ||
		errors.Is(err, user.UnknownGroupIdError(groupName)) ||
		strings.HasSuffix(err.Error(), syscall.ENOENT.Error()) ||
		strings.HasSuffix(err.Error(), syscall.ESRCH.Error())
}

// DeleteAllUsers deletes all host users in the teleport service group.
func (u *HostUserManagement) DeleteAllUsers() error {
	users, err := u.backend.GetAllUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	teleportGroup, err := u.backend.LookupGroup(types.TeleportServiceGroup)
	if err != nil {
		if isUnknownGroupError(err, types.TeleportServiceGroup) {
			log.Debugf("%q group not found, not deleting users", types.TeleportServiceGroup)
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

// DeleteUser deletes the specified user only if they are
// present in the specified group.
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
		err := u.DeleteAllUsers()
		switch {
		case trace.IsNotFound(err):
			log.Debugf("Error during temporary user cleanup: %s, stopping cleanup job", err)
			return
		case err != nil:
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

// UserExists looks up an existing host user.
func (u *HostUserManagement) UserExists(username string) error {
	_, err := u.backend.Lookup(username)
	if err != nil {
		if errors.Is(err, user.UnknownUserError(username)) {
			return trace.NotFound("User not found: %s", err)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (u *HostUserManagement) SetHostUserDeletionGrace(d time.Duration) {
	u.userGrace = d
}
