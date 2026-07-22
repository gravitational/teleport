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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ui"
)

// UserGroup describes a user group.
type UserGroup struct {
	// Name is the name of the group.
	Name string `json:"name"`
	// Description is the description of the group.
	Description string `json:"description"`
	// Labels is the user group list of labels
	Labels []ui.Label `json:"labels"`
	// FriendlyName is a friendly name for the user group.
	FriendlyName string `json:"friendlyName,omitempty"`
	// Applications is a list of associated applications.
	Applications []ApplicationAndFriendlyName `json:"applications,omitempty"`
}

// ApplicationAndFriendlyName is an application name and its friendly name.
type ApplicationAndFriendlyName struct {
	// Name is the name of the application.
	Name string `json:"name"`
	// FriendlyName is the friendly name of the application.
	FriendlyName string `json:"friendlyName"`
}

// MakeUserGroups creates user group objects for the UI.
func MakeUserGroups(userGroups []types.UserGroup, userGroupsToApps map[string]types.Apps) ([]UserGroup, error) {
	uiUserGroups := []UserGroup{}
	for _, userGroup := range userGroups {
		uiLabels := ui.MakeLabelsWithoutInternalPrefixes(userGroup.GetStaticLabels())

		apps := userGroupsToApps[userGroup.GetName()]
		appsAndFriendlyNames := make([]ApplicationAndFriendlyName, len(apps))
		for i, app := range apps {
			appsAndFriendlyNames[i] = ApplicationAndFriendlyName{
				Name:         app.GetName(),
				FriendlyName: types.FriendlyName(app),
			}
		}

		// Use the explicitly set Okta label if it's present.
		description := userGroup.GetMetadata().Description
		if oktaDescription, ok := userGroup.GetLabel(types.OktaGroupDescriptionLabel); ok {
			description = oktaDescription
		}

		uiUserGroups = append(uiUserGroups, UserGroup{
			Name:         userGroup.GetName(),
			Description:  description,
			Labels:       uiLabels,
			FriendlyName: types.FriendlyName(userGroup),
			Applications: appsAndFriendlyNames,
		})
	}

	return uiUserGroups, nil
}
