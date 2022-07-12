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

// User contains data needed by the web UI to display locally saved users.
type User struct {
	UserListEntry
	// Logins is the list of Logins Trait for the user
	Logins []string `json:"logins,omitempty"`
	// DatabaseUsers is the list of DatabaseUsers Trait for the user
	DatabaseUsers []string `json:"database_users,omitempty"`
	// DatabaseNames is the list of DatabaseNames Trait for the user
	DatabaseNames []string `json:"database_names,omitempty"`
	// KubeUsers is the list of KubeUsers Trait for the user
	KubeUsers []string `json:"kube_users,omitempty"`
	// KubeGroups is the list of KubeGroups Trait for the user
	KubeGroups []string `json:"kube_groups,omitempty"`
	// WindowsLogins is the list of WindowsLogins Trait for the user
	WindowsLogins []string `json:"windows_logins,omitempty"`
	// AWSRoleARNs is the list of AWSRoleARNs Trait for the user
	AWSRoleARNs []string `json:"aws_role_ar_ns,omitempty"`
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
		Logins:        teleUser.GetLogins(),
		DatabaseUsers: teleUser.GetDatabaseUsers(),
		DatabaseNames: teleUser.GetDatabaseNames(),
		KubeUsers:     teleUser.GetKubeUsers(),
		KubeGroups:    teleUser.GetKubeGroups(),
		WindowsLogins: teleUser.GetWindowsLogins(),
		AWSRoleARNs:   teleUser.GetAWSRoleARNs(),
	}, nil
}
