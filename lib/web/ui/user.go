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

package ui

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type UserListEntry struct {
	// Name is the user name.
	Name string `json:"name"`
	// Roles is the list of roles user belongs to.
	Roles []string `json:"roles"`
	// AuthType is the type of auth service
	// that the user was authenticated through.
	AuthType string `json:"authType"`
	// AllTraits returns all the traits.
	// Different from "userTraits" where "userTraits"
	// "selectively" returns traits.
	AllTraits map[string][]string `json:"allTraits"`
	// Origin is the type of upstream IdP that manages the user. May be empty.
	Origin string `json:"origin"`
	// IsBot is true if the user is a Bot User.
	IsBot bool `json:"isBot"`
}

type userTraits struct {
	// Logins is the list of logins that a user is
	// allowed to start SSH sessions with.
	Logins []string `json:"logins,omitempty"`
	// DatabaseUsers is the list of db usernames that a
	// user is allowed to open db connections as.
	DatabaseUsers []string `json:"databaseUsers,omitempty"`
	// DatabaseNames is the list of db names that a user can connect to.
	DatabaseNames []string `json:"databaseNames,omitempty"`
	// KubeUsers is the list of allowed kube logins.
	KubeUsers []string `json:"kubeUsers,omitempty"`
	// KubeGroups is the list of KubeGroups Trait for the user.
	KubeGroups []string `json:"kubeGroups,omitempty"`
	// WindowsLogins is the list of logins that this user
	// is allowed to start desktop sessions.
	WindowsLogins []string `json:"windowsLogins,omitempty"`
	// AWSRoleARNs is a list of aws roles this user is allowed to assume.
	AWSRoleARNs []string `json:"awsRoleArns,omitempty"`
}

// User contains data needed by the web UI to display locally saved users.
type User struct {
	UserListEntry
	// Traits contain select fields that define traits for local accounts.
	Traits userTraits `json:"traits"`
}

func NewUserListEntry(teleUser types.User) (*UserListEntry, error) {
	if teleUser == nil {
		return nil, trace.BadParameter("missing teleUser")
	}

	authType := "local"
	if teleUser.GetUserType() == types.UserTypeSSO {
		authType = teleUser.GetCreatedBy().Connector.Type
	}

	return &UserListEntry{
		Name:      teleUser.GetName(),
		Roles:     teleUser.GetRoles(),
		AuthType:  authType,
		Origin:    teleUser.Origin(),
		AllTraits: teleUser.GetTraits(),
		IsBot:     teleUser.IsBot(),
	}, nil
}

// NewUser creates UI user object
func NewUser(teleUser types.User) (*User, error) {
	// NewUserListEntry checks for a nil teleUser, no need to check for it here

	userListEntry, err := NewUserListEntry(teleUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &User{
		UserListEntry: *userListEntry,
		Traits: userTraits{
			Logins:        teleUser.GetLogins(),
			DatabaseUsers: teleUser.GetDatabaseUsers(),
			DatabaseNames: teleUser.GetDatabaseNames(),
			KubeUsers:     teleUser.GetKubeUsers(),
			KubeGroups:    teleUser.GetKubeGroups(),
			WindowsLogins: teleUser.GetWindowsLogins(),
			AWSRoleARNs:   teleUser.GetAWSRoleARNs(),
		},
	}, nil
}
