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

package ui

import "github.com/gravitational/teleport/api/types"

// OktaApp describes an Okta application
type OktaApp struct {
	// Name is the name of the Okta application.
	Name string `json:"name"`
	// Description is the Okta app description.
	Description string `json:"description"`
	// ApplicationID is the application ID of the Okta application.
	ApplicationID string `json:"application_id"`
	// Labels is a map of static labels associated with an Okta application.
	Labels []Label `json:"labels"`
}

// MakeOktaApps creates a UI representation of Okta applications.
func MakeOktaApps(oktaApps []types.OktaApplication) []OktaApp {
	uiOktaApps := make([]OktaApp, len(oktaApps))

	for i, oktaApp := range oktaApps {
		uiOktaApps[i] = OktaApp{
			Name:          oktaApp.GetName(),
			Description:   oktaApp.GetDescription(),
			ApplicationID: oktaApp.GetApplicationID(),
			Labels:        makeLabels(oktaApp.GetAllLabels()),
		}
	}

	return uiOktaApps
}

// OktaGroup describes an Okta group
type OktaGroup struct {
	// Name is the name of the Okta group.
	Name string `json:"name"`
	// Labels is a map of static labels associated with an Okta group.
	Labels []Label `json:"labels"`
}

// MakeOktaGroups creates a UI representation of Okta groups.
func MakeOktaGroup(oktaGroups []types.OktaGroup) []OktaGroup {
	uiOktaGroups := make([]OktaGroup, len(oktaGroups))

	for i, oktaGroup := range oktaGroups {
		uiOktaGroups[i] = OktaGroup{
			Name:   oktaGroup.GetName(),
			Labels: makeLabels(oktaGroup.GetAllLabels()),
		}
	}

	return uiOktaGroups
}
