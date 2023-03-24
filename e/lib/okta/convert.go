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

package okta

import (
	"encoding/base64"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport/api/types"
)

const (
	oktaActive       = "ACTIVE"
	oktaAdminConsole = "Okta Admin Console"
	oktaOrgURLLabel  = "okta/org"
)

// oktaGroupToUserGroup converts an Okta group object to a types.UserGroup object.
func (s *Service) oktaGroupToUserGroup(oktaGroup *okta.Group) (types.UserGroup, error) {
	if oktaGroup.Profile == nil {
		return nil, trace.BadParameter("the okta group object has no profile")
	}

	labels := s.getGroupLabels(oktaGroup.Id)
	labels[types.OriginLabel] = types.OriginOkta
	labels[oktaOrgURLLabel] = s.orgURL

	userGroup, err := types.NewUserGroup(
		types.Metadata{
			Name:        oktaGroup.Id,
			Description: oktaGroup.Profile.Description,
			Labels:      labels,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userGroup, nil
}

type links struct {
	AppLinks []appLinks `mapstructure:"appLinks"`
}

type appLinks struct {
	Name string `mapstructure:"name"`
	Href string `mapstructure:"href"`
}

// oktaAppToApps converts an Okta app object to types.Application objects. This will convert
// multiple appLinks in an Okta object into multiple applications.
func (s *Service) oktaAppToApps(oktaApp okta.App) ([]types.Application, error) {
	// This type assertion is necessary as okta.App, which is supplied by the Okta go SDK,
	// does not contain all of the information that we need to create a types.Application
	// object.
	oktaAppFromAPI, ok := oktaApp.(*okta.Application)
	if !ok {
		return nil, trace.BadParameter("unable infer type of of Okta application: %T", oktaApp)
	}

	appIdentifier := fmt.Sprintf("%s (%s)", oktaAppFromAPI.Id, oktaAppFromAPI.Label)

	// Filter out Okta apps if they're not the kind we want to display to users..
	if err := isAppValid(oktaAppFromAPI); err != nil {
		return nil, trace.Wrap(err)
	}

	if oktaAppFromAPI.Links == nil {
		return nil, trace.BadParameter("links is missing in okta application object %s", appIdentifier)
	}

	// Unfortunately the app links are stuffed into an interface{}, so we've got to extract the
	// fields for app links using mapstructure.
	links := &links{}
	err := mapstructure.Decode(oktaAppFromAPI.Links, links)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(links.AppLinks) == 0 {
		return nil, trace.BadParameter("app links is empty in okta application object %s", appIdentifier)
	}

	var applications []types.Application

	labels := s.getApplicationLabels(oktaAppFromAPI.Id)
	labels[types.OriginLabel] = types.OriginOkta
	labels[oktaOrgURLLabel] = s.orgURL

	// Create an application for each app link. This is required because there can be multiple
	// app links per Okta application.
	for _, appLink := range links.AppLinks {
		appID, err := s.appName(oktaAppFromAPI.Id, appLink.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		app, err := types.NewAppV3(
			types.Metadata{
				Name:        appID,
				Description: oktaAppFromAPI.Label,
				Labels:      labels,
			},
			types.AppSpecV3{
				URI: appLink.Href,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		applications = append(applications, app)
	}

	return applications, nil
}

// appName returns an app name based on the ID and app link name.
func (s *Service) appName(id, appLinkName string) (string, error) {
	// Let's create a short unique string for the app ID.
	hasher := s.hash.New()
	_, err := hasher.Write([]byte(fmt.Sprintf("%s-%s", id, appLinkName)))
	if err != nil {
		return "", trace.Wrap(err)
	}
	hashedID := hasher.Sum(nil)

	// Take only the first 12 characters of the resulting base64 encoded hash.
	// Otherwise, the links for the apps in the UI are far too long.
	return base64.RawURLEncoding.EncodeToString(hashedID)[:12], nil
}

// isAppValid will return an error if the application shouldn't be synced with the app catalog.
func isAppValid(app *okta.Application) error {
	appIdentifier := fmt.Sprintf("%s (%s)", app.Id, app.Label)

	// If the application isn't active, then we'll filter it out.
	if app.Status != oktaActive {
		return trace.BadParameter("application %s is not active", appIdentifier)
	}

	// We'll filter out the admin console as well.
	if app.Label == oktaAdminConsole {
		return trace.BadParameter("application %s is the Okta admin console", app.Id)
	}

	// Make sure the app isn't hidden.
	if app.Visibility != nil && app.Visibility.Hide != nil && app.Visibility.Hide.Web != nil && *app.Visibility.Hide.Web {
		return trace.BadParameter("application %s is hidden from the web", appIdentifier)
	}

	return nil
}
