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
	"sort"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws"
)

// App describes an application
type App struct {
	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
	Kind string `json:"kind"`
	// Name is the name of the application.
	Name string `json:"name"`
	// Description is the app description.
	Description string `json:"description"`
	// URI is the internal address the application is available at.
	URI string `json:"uri"`
	// PublicAddr is the public address the application is accessible at.
	PublicAddr string `json:"publicAddr"`
	// FQDN is a fully qualified domain name of the application (app.example.com)
	FQDN string `json:"fqdn"`
	// ClusterID is this app cluster ID
	ClusterID string `json:"clusterId"`
	// Labels is a map of static labels associated with an application.
	Labels []Label `json:"labels"`
	// AWSConsole if true, indicates that the app represents AWS management console.
	AWSConsole bool `json:"awsConsole"`
	// AWSRoles is a list of AWS IAM roles for the application representing AWS console.
	AWSRoles []aws.Role `json:"awsRoles,omitempty"`
	// FriendlyName is a friendly name for the app.
	FriendlyName string `json:"friendlyName,omitempty"`
	// UserGroups is a list of associated user groups.
	UserGroups []UserGroupAndDescription `json:"userGroups,omitempty"`
	// SAMLApp if true, indicates that the app is a SAML Application (SAML IdP Service Provider)
	SAMLApp bool `json:"samlApp,omitempty"`
	// Integration is the integration name that must be used to access this Application.
	// Only applicable to AWS App Access.
	Integration string `json:"integration,omitempty"`
}

// UserGroupAndDescription is a user group name and its description.
type UserGroupAndDescription struct {
	// Name is the name of the user group.
	Name string `json:"name"`
	// Description is the description of the user group.
	Description string `json:"description"`
}

// MakeAppsConfig contains parameters for converting apps to UI representation.
type MakeAppsConfig struct {
	// LocalClusterName is the name of the local cluster.
	LocalClusterName string
	// LocalProxyDNSName is the public hostname of the local cluster.
	LocalProxyDNSName string
	// AppClusterName is the name of the cluster apps reside in.
	AppClusterName string
	// AppsToUserGroups is a mapping of application names to user groups.
	AppsToUserGroups map[string]types.UserGroups
	// AppServersAndSAMLIdPServiceProviders is a list of AppServers and SAMLIdPServiceProviders.
	AppServersAndSAMLIdPServiceProviders types.AppServersOrSAMLIdPServiceProviders
	// AllowedAWSRolesLookup is a map of AWS IAM Role ARNs available to each App for the logged user.
	// Only used for AWS Console Apps.
	AllowedAWSRolesLookup map[string][]string
	// UserGroupLookup is a map of user groups to provide to each App
	UserGroupLookup map[string]types.UserGroup
	// Logger is a logger used for debugging while making an app
	Logger logrus.FieldLogger
}

// MakeApp creates an application object for the WebUI.
func MakeApp(app types.Application, c MakeAppsConfig) App {
	labels := makeLabels(app.GetAllLabels())
	fqdn := utils.AssembleAppFQDN(c.LocalClusterName, c.LocalProxyDNSName, c.AppClusterName, app)
	var ugs types.UserGroups
	for _, userGroupName := range app.GetUserGroups() {
		userGroup := c.UserGroupLookup[userGroupName]
		if userGroup == nil {
			c.Logger.Debugf("Unable to find user group %s when creating user groups, skipping", userGroupName)
			continue
		}

		ugs = append(ugs, userGroup)
	}
	sort.Sort(ugs)

	userGroupAndDescriptions := make([]UserGroupAndDescription, len(ugs))
	for i, userGroup := range ugs {
		userGroupAndDescriptions[i] = UserGroupAndDescription{
			Name:        userGroup.GetName(),
			Description: userGroup.GetMetadata().Description,
		}
	}

	// Use the explicitly set Okta label if it's present.
	description := app.GetMetadata().Description
	if oktaDescription, ok := app.GetLabel(types.OktaAppDescriptionLabel); ok {
		description = oktaDescription
	}

	resultApp := App{
		Kind:         types.KindApp,
		Name:         app.GetName(),
		Description:  description,
		URI:          app.GetURI(),
		PublicAddr:   app.GetPublicAddr(),
		Labels:       labels,
		ClusterID:    c.AppClusterName,
		FQDN:         fqdn,
		AWSConsole:   app.IsAWSConsole(),
		FriendlyName: types.FriendlyName(app),
		UserGroups:   userGroupAndDescriptions,
		SAMLApp:      false,
		Integration:  app.GetIntegration(),
	}

	if app.IsAWSConsole() {
		allowedAWSRoles := c.AllowedAWSRolesLookup[app.GetName()]
		resultApp.AWSRoles = aws.FilterAWSRoles(allowedAWSRoles,
			app.GetAWSAccountID())
	}

	return resultApp
}

// MakeSAMLApp creates a SAMLIdPServiceProvider object for the WebUI.
// Keep in sync with lib/teleterm/apiserver/handler/handler_apps.go.
func MakeSAMLApp(app types.SAMLIdPServiceProvider, c MakeAppsConfig) App {
	labels := makeLabels(app.GetAllLabels())
	resultApp := App{
		Kind:         types.KindApp,
		Name:         app.GetName(),
		Description:  "SAML Application",
		PublicAddr:   "",
		Labels:       labels,
		ClusterID:    c.AppClusterName,
		FriendlyName: types.FriendlyName(app),
		SAMLApp:      true,
	}

	return resultApp
}

// MakeApps creates application objects (either Application Servers or SAML IdP Service Provider) for the WebUI.
func MakeApps(c MakeAppsConfig) []App {
	result := []App{}
	for _, appOrSP := range c.AppServersAndSAMLIdPServiceProviders {
		if appOrSP.IsAppServer() {
			app := appOrSP.GetAppServer().GetApp()
			fqdn := utils.AssembleAppFQDN(c.LocalClusterName, c.LocalProxyDNSName, c.AppClusterName, app)
			labels := makeLabels(app.GetAllLabels())

			userGroups := c.AppsToUserGroups[app.GetName()]

			userGroupAndDescriptions := make([]UserGroupAndDescription, len(userGroups))
			for i, userGroup := range userGroups {
				userGroupAndDescriptions[i] = UserGroupAndDescription{
					Name:        userGroup.GetName(),
					Description: userGroup.GetMetadata().Description,
				}
			}

			resultApp := App{
				Kind:         types.KindApp,
				Name:         appOrSP.GetName(),
				Description:  appOrSP.GetDescription(),
				URI:          app.GetURI(),
				PublicAddr:   appOrSP.GetPublicAddr(),
				Labels:       labels,
				ClusterID:    c.AppClusterName,
				FQDN:         fqdn,
				AWSConsole:   app.IsAWSConsole(),
				FriendlyName: types.FriendlyName(app),
				UserGroups:   userGroupAndDescriptions,
				SAMLApp:      false,
			}

			if app.IsAWSConsole() {
				allowedAWSRoles := c.AllowedAWSRolesLookup[app.GetName()]
				resultApp.AWSRoles = aws.FilterAWSRoles(allowedAWSRoles,
					app.GetAWSAccountID())
			}

			result = append(result, resultApp)
		} else {
			labels := makeLabels(appOrSP.GetSAMLIdPServiceProvider().GetAllLabels())
			resultApp := App{
				Kind:         types.KindApp,
				Name:         appOrSP.GetName(),
				Description:  appOrSP.GetDescription(),
				PublicAddr:   appOrSP.GetPublicAddr(),
				Labels:       labels,
				ClusterID:    c.AppClusterName,
				FriendlyName: types.FriendlyName(appOrSP),
				SAMLApp:      true,
			}

			result = append(result, resultApp)
		}
	}

	return result
}
