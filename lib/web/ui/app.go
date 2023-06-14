/*
Copyright 2020 Gravitational, Inc.

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
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/aws"
)

// App describes an application
type App struct {
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
}

// MakeAppsConfig contains parameters for converting apps to UI representation.
type MakeAppsConfig struct {
	// LocalClusterName is the name of the local cluster.
	LocalClusterName string
	// LocalProxyDNSName is the public hostname of the local cluster.
	LocalProxyDNSName string
	// AppClusterName is the name of the cluster apps reside in.
	AppClusterName string
	// Apps is a list of registered apps.
	Apps types.Apps
	// Identity is identity of the logged in user.
	Identity *tlsca.Identity
}

// MakeApps creates server application objects
func MakeApps(c MakeAppsConfig) []App {
	result := []App{}
	for _, teleApp := range c.Apps {
		fqdn := AssembleAppFQDN(c.LocalClusterName, c.LocalProxyDNSName, c.AppClusterName, teleApp)
		labels := makeLabels(teleApp.GetAllLabels())

		app := App{
			Name:         teleApp.GetName(),
			Description:  teleApp.GetDescription(),
			URI:          teleApp.GetURI(),
			PublicAddr:   teleApp.GetPublicAddr(),
			Labels:       labels,
			ClusterID:    c.AppClusterName,
			FQDN:         fqdn,
			AWSConsole:   teleApp.IsAWSConsole(),
			FriendlyName: services.FriendlyName(teleApp),
		}

		if teleApp.IsAWSConsole() {
			app.AWSRoles = aws.FilterAWSRoles(c.Identity.AWSRoleARNs,
				teleApp.GetAWSAccountID())
		}

		result = append(result, app)
	}

	return result
}

// AssembleAppFQDN returns the application's FQDN.
//
// If the application is running within the local cluster and it has a public
// address specified, the application's public address is used.
//
// In all other cases, i.e. if the public address is not set or the application
// is running in a remote cluster, the FQDN is formatted as
// <appName>.<localProxyDNSName>
func AssembleAppFQDN(localClusterName string, localProxyDNSName string, appClusterName string, app types.Application) string {
	isLocalCluster := localClusterName == appClusterName
	if isLocalCluster && app.GetPublicAddr() != "" {
		return app.GetPublicAddr()
	}
	return fmt.Sprintf("%v.%v", app.GetName(), localProxyDNSName)
}
