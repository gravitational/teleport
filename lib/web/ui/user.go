/*
Copyright 2021 Gravitational, Inc.

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

package ui

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type UserListEntry struct {
	// Name is the user name.
	Name string `json:"name"`
	// Roles is the list of roles user belongs to.
	Roles []string `json:"roles"`
	// AuthType is the type of auth service
	// that the user was authenticated through.
	AuthType string `json:"authType"`
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
	// Traits contain fields that define traits for local accounts.
	Traits userTraits `json:"traits"`
}

func NewUserListEntry(teleUser types.User) (*UserListEntry, error) {
	if teleUser == nil {
		return nil, trace.BadParameter("missing teleUser")
	}

	authType := "local"
	if teleUser.GetCreatedBy().Connector != nil {
		authType = teleUser.GetCreatedBy().Connector.Type
	}

	return &UserListEntry{
		Name:     teleUser.GetName(),
		Roles:    teleUser.GetRoles(),
		AuthType: authType,
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
