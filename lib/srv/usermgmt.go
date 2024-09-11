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
	"log/slog"
	"maps"
	"os/user"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/host"
)

// NewHostUsers initialize a new HostUsers object
func NewHostUsers(ctx context.Context, storage services.PresenceInternal, uuid string) HostUsers {
	//nolint:staticcheck // SA4023. False positive on macOS.
	backend, err := newHostUsersBackend()
	switch {
	case trace.IsNotImplemented(err):
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
	CreateUser(name string, groups []string, opts host.UserOpts) error
	// DeleteUser deletes a user from a host.
	DeleteUser(name string) error
	// CreateHomeDirectory creates the users home directory and copies in /etc/skel
	CreateHomeDirectory(userHome string, uid, gid string) error
	// GetDefaultHomeDirectory returns the default home directory path for the given user
	GetDefaultHomeDirectory(user string) (string, error)
}

type userCloser struct {
	users    HostUsers
	backend  HostUsersBackend
	username string
}

func (u *userCloser) Close() error {
	teleportGroup, err := u.backend.LookupGroup(types.TeleportDropGroup)
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
	// WriteSudoers creates a temporary Teleport user in the TeleportDropGroup
	WriteSudoers(name string, sudoers []string) error
	// RemoveSudoers removes the users sudoer file
	RemoveSudoers(name string) error
}

type HostSudoersNotImplemented struct{}

// WriteSudoers creates a temporary Teleport user in the TeleportDropGroup
func (*HostSudoersNotImplemented) WriteSudoers(string, []string) error {
	return trace.NotImplemented("host sudoers functionality not implemented on this platform")
}

// RemoveSudoers removes the users sudoer file
func (*HostSudoersNotImplemented) RemoveSudoers(name string) error {
	return trace.NotImplemented("host sudoers functionality not implemented on this platform")
}

type HostUsers interface {
	// UpsertUser creates a temporary Teleport user in the TeleportDropGroup
	UpsertUser(name string, hostRoleInfo services.HostUsersInfo) (io.Closer, error)
	// DeleteUser deletes a temporary Teleport user only if they are
	// in a specified group
	DeleteUser(name string, gid string) error
	// DeleteAllUsers deletes all suer in the TeleportDropGroup
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

// unmanagedUserErr is returned when attempting to modify or interact with a user that is not managed by Teleport.
var unmanagedUserErr = errors.New("user not managed by teleport")

func (u *HostUserManagement) updateUser(name string, ui services.HostUsersInfo) (io.Closer, error) {
	existingUser, err := u.backend.Lookup(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currentGroups := make(map[string]struct{}, len(ui.Groups))
	groupIDs, err := u.backend.UserGIDs(existingUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, groupID := range groupIDs {
		group, err := u.backend.LookupGroupByID(groupID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		currentGroups[group.Name] = struct{}{}
	}

	// allow for explicit assignment of teleport-keep group in order to facilitate migrating KEEP users that existed before we added
	// the teleport-keep group
	migrateKeepUser := slices.Contains(ui.Groups, types.TeleportKeepGroup)

	_, hasDropGroup := currentGroups[types.TeleportDropGroup]
	_, hasKeepGroup := currentGroups[types.TeleportKeepGroup]
	_, hasStaticGroup := currentGroups[types.TeleportStaticGroup]
	if !(hasDropGroup || hasKeepGroup || hasStaticGroup || migrateKeepUser) {
		return nil, trace.Errorf("%q %w", name, unmanagedUserErr)
	}

	// Do not convert/update groups from static to non-static user, and vice versa.
	isStaticUser := ui.Mode == services.HostUserModeStatic
	if hasStaticGroup != isStaticUser {
		slog.DebugContext(context.Background(),
			"Aborting host user creation, can't convert between auto-provisioned and static host users.",
			"login", name)
		return nil, nil
	}

	var closer io.Closer
	switch ui.Mode {
	case services.HostUserModeDrop:
		ui.Groups = append(ui.Groups, types.TeleportDropGroup)
		closer = &userCloser{
			username: name,
			users:    u,
			backend:  u.backend,
		}
	case services.HostUserModeStatic:
		ui.Groups = append(ui.Groups, types.TeleportStaticGroup)
	case services.HostUserModeKeep:
		if !hasKeepGroup {
			home, err := u.backend.GetDefaultHomeDirectory(name)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if err := u.backend.CreateHomeDirectory(home, existingUser.Uid, existingUser.Gid); err != nil {
				return nil, trace.Wrap(err)
			}
		}

		// no need to duplicate the group if it's already there
		if !migrateKeepUser {
			ui.Groups = append(ui.Groups, types.TeleportKeepGroup)
		}
	}

	finalGroups := make(map[string]struct{}, len(ui.Groups))
	for _, group := range ui.Groups {
		finalGroups[group] = struct{}{}
	}

	primaryGroup, err := u.backend.LookupGroupByID(existingUser.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	finalGroups[primaryGroup.Name] = struct{}{}

	if !maps.Equal(currentGroups, finalGroups) {
		if err := u.doWithUserLock(func(_ types.SemaphoreLease) error {
			return trace.Wrap(u.backend.SetUserGroups(name, ui.Groups))
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return closer, nil
}

func (u *HostUserManagement) resolveGID(username string, groups []string, gid string) (string, error) {
	if gid != "" {
		// ensure user's primary group exists if a gid is explicitly provided
		err := u.backend.CreateGroup(username, gid)
		if err != nil && !trace.IsAlreadyExists(err) {
			return "", trace.Wrap(err)
		}

		return gid, nil
	}

	// user's without an explicit gid should use the group that shares their login
	// name if defined, otherwise user creation will fail due to their primary group
	// already existing
	if slices.Contains(groups, username) {
		return username, nil
	}

	// avoid automatic assignment of groups not defined in the role
	if _, err := u.backend.LookupGroup(username); err == nil {
		return "", trace.AlreadyExists("host login %q conflicts with an existing group that is not defined in user's role, either add %q to host_groups or explicitly assign a GID", username, username)
	}

	return "", nil
}

func (u *HostUserManagement) createUser(name string, ui services.HostUsersInfo) (io.Closer, error) {
	var err error
	userOpts := host.UserOpts{
		UID:   ui.UID,
		GID:   ui.GID,
		Shell: ui.Shell,
	}

	var closer io.Closer
	switch ui.Mode {
	case services.HostUserModeDrop:
		ui.Groups = append(ui.Groups, types.TeleportDropGroup)
		closer = &userCloser{
			username: name,
			users:    u,
			backend:  u.backend,
		}
	case services.HostUserModeKeep, services.HostUserModeStatic:
		if ui.Mode == services.HostUserModeStatic {
			ui.Groups = append(ui.Groups, types.TeleportStaticGroup)
		} else {
			ui.Groups = append(ui.Groups, types.TeleportKeepGroup)
		}
		userOpts.Home, err = u.backend.GetDefaultHomeDirectory(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	err = u.doWithUserLock(func(_ types.SemaphoreLease) error {
		if ui.Mode == services.HostUserModeDrop {
			if err := u.storage.UpsertHostUserInteractionTime(u.ctx, name, time.Now()); err != nil {
				return trace.Wrap(err)
			}
		}

		userOpts.GID, err = u.resolveGID(name, ui.Groups, ui.GID)
		if err != nil {
			return trace.Wrap(err)
		}

		err = u.backend.CreateUser(name, ui.Groups, userOpts)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.WrapWithMessage(err, "error while creating user")
		}

		user, err := u.backend.Lookup(name)
		if err != nil {
			return trace.Wrap(err)
		}

		if userOpts.Home != "" {
			if err := u.backend.CreateHomeDirectory(userOpts.Home, user.Uid, user.Gid); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return closer, nil
}

func (u *HostUserManagement) ensureGroupsExist(groups ...string) error {
	var errs []error

	for _, group := range groups {
		if err := u.createGroupIfNotExist(group); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}

// UpsertUser creates a temporary Teleport user in the TeleportDropGroup
func (u *HostUserManagement) UpsertUser(name string, ui services.HostUsersInfo) (io.Closer, error) {
	// allow for explicit assignment of teleport-keep group in order to facilitate migrating KEEP users that existed before we added
	// the teleport-keep group
	migrateKeepUser := slices.Contains(ui.Groups, types.TeleportKeepGroup)
	skipKeepGroup := migrateKeepUser && ui.Mode != services.HostUserModeKeep

	if skipKeepGroup {
		log.Warnf("explicit assignment of %q group is not possible in 'insecure-drop' mode", types.TeleportKeepGroup)
	}

	groupSet := make(map[string]struct{}, len(ui.Groups))
	groups := make([]string, 0, len(ui.Groups))
	for _, group := range ui.Groups {
		switch group {
		// TeleportDropGroup and TeleportStaticGroup are managed automatically and should not be allowed direct assignment
		case types.TeleportDropGroup, types.TeleportStaticGroup:
			continue
		case types.TeleportKeepGroup:
			if skipKeepGroup {
				continue
			}
		}

		if _, ok := groupSet[group]; !ok {
			groupSet[group] = struct{}{}
			groups = append(groups, group)
		}
	}
	ui.Groups = groups

	if err := u.ensureGroupsExist(types.TeleportDropGroup, types.TeleportKeepGroup, types.TeleportStaticGroup); err != nil {
		return nil, trace.WrapWithMessage(err, "creating teleport system groups")
	}

	if err := u.ensureGroupsExist(groups...); err != nil {
		return nil, trace.WrapWithMessage(err, "creating configured groups")
	}

	closer, err := u.updateUser(name, ui)
	if err != nil {
		if !errors.Is(err, user.UnknownUserError(name)) {
			return nil, trace.Wrap(err)
		}

		if closer, err = u.createUser(name, ui); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return closer, nil
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

	return trace.Wrap(err, "%q", group)
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
	teleportGroup, err := u.backend.LookupGroup(types.TeleportDropGroup)
	if err != nil {
		if isUnknownGroupError(err, types.TeleportDropGroup) {
			log.Debugf("%q group not found, not deleting users", types.TeleportDropGroup)
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
