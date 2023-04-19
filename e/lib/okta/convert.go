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
	"crypto"
	"encoding/base64"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/srv/app"
)

const (
	oktaActive       = "ACTIVE"
	oktaAdminConsole = "Okta Admin Console"
)

// oktaGroupToUserGroup converts an Okta group object to a types.UserGroup object.
func (s *Service) oktaGroupToUserGroup(oktaGroup *okta.Group) (types.UserGroup, error) {
	if oktaGroup.Profile == nil {
		return nil, trace.BadParameter("the okta group object has no profile")
	}

	labels := s.getGroupLabels(oktaGroup.Id)
	labels[types.OriginLabel] = types.OriginOkta
	labels[teleport.OktaOrgURLLabel] = s.orgURL
	labels[teleport.OktaGroupIDLabel] = oktaGroup.Id

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

type embeddedLinks struct {
	AppLinks []appLinks `mapstructure:"appLinks"`
}

type appLinks struct {
	Name string `mapstructure:"name"`
	Href string `mapstructure:"href"`
}

// oktaAppToApps converts an Okta app object to types.Application objects. This will convert
// multiple appLinks in an Okta object into multiple applications.
func (s *Service) oktaAppToApp(oktaApplication *okta.Application) ([]*types.AppV3, error) {
	appIdentifier := fmt.Sprintf("%s (%s)", oktaApplication.Id, oktaApplication.Label)

	// Filter out Okta apps if they're not the kind we want to display to users..
	if err := isAppValid(oktaApplication); err != nil {
		return nil, trace.Wrap(err)
	}

	if oktaApplication.Links == nil {
		return nil, trace.BadParameter("links is missing in okta application object %s", appIdentifier)
	}

	// Unfortunately the app links are stuffed into an interface{}, so we've got to extract the
	// fields for app links using mapstructure.
	embeddedLinks := &embeddedLinks{}
	if err := mapstructure.Decode(oktaApplication.Links, embeddedLinks); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(embeddedLinks.AppLinks) == 0 {
		return nil, trace.BadParameter("app links is empty in okta application object %s", appIdentifier)
	}

	var apps []*types.AppV3

	labels := s.getApplicationLabels(oktaApplication.Id)
	labels[types.OriginLabel] = types.OriginOkta
	labels[teleport.OktaOrgURLLabel] = s.orgURL
	labels[teleport.OktaAppIDLabel] = oktaApplication.Id

	// Create an app for each app link. This is required because there can be multiple
	// app links per Okta application.
	for _, appLink := range embeddedLinks.AppLinks {
		appID, err := appName(s.hash, oktaApplication.Id, appLink.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		publicAddr, err := app.FindPublicAddr(s.accessPoint, "", appID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		app, err := types.NewAppV3(
			types.Metadata{
				Name:        appID,
				Description: oktaApplication.Label,
				Labels:      labels,
			},
			types.AppSpecV3{
				URI:        appLink.Href,
				PublicAddr: publicAddr,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// appName returns an app name based on the ID and app link name.
func appName(hash crypto.Hash, id, appLinkName string) (string, error) {
	// Let's create a short unique string for the app ID.
	hasher := hash.New()
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
