/*
Copyright 2015 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/services"
)

// UserGroup describes a user group.
type UserGroup struct {
	// Name is the name of the group.
	Name string `json:"name"`
	// Description is the description of the group.
	Description string `json:"description"`
	// Labels is the user group list of labels
	Labels []Label `json:"labels"`
}

// MakeUserGroups creates user group objects for the UI.
func MakeUserGroups(clusterName string, userGroups []types.UserGroup, userRoles services.RoleSet) ([]UserGroup, error) {
	uiUserGroups := []UserGroup{}
	for _, userGroup := range userGroups {
		uiLabels := makeLabels(userGroup.GetStaticLabels())

		uiUserGroups = append(uiUserGroups, UserGroup{
			Name:        userGroup.GetName(),
			Description: userGroup.GetMetadata().Description,
			Labels:      uiLabels,
		})
	}

	return uiUserGroups, nil
}
