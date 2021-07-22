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

// User contains data needed by the web UI to display locally saved users.
type User struct {
	// Name is the user name.
	Name string `json:"name"`
	// Roles is the list of roles user belongs to.
	Roles []string `json:"roles"`
	// AuthType is the type of auth service
	// that the user was authenticated through.
	AuthType string `json:"authType"`
}

// NewUser creates UI user object
func NewUser(teleUser types.User) (*User, error) {
	if teleUser == nil {
		return nil, trace.BadParameter("missing teleUser")
	}

	authType := "local"
	if teleUser.GetCreatedBy().Connector != nil {
		authType = teleUser.GetCreatedBy().Connector.Type
	}

	return &User{
		Name:     teleUser.GetName(),
		Roles:    teleUser.GetRoles(),
		AuthType: authType,
	}, nil
}
