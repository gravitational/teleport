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
	"sort"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// App describes an application
type App struct {
	// Name is the name of the application.
	Name string `json:"name"`
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
}

// MakeApps creates server application objects
func MakeApps(localClusterName string, localProxyDNSName string, appClusterName string, appServers []services.Server) []App {
	result := []App{}
	for _, server := range appServers {
		teleApps := server.GetApps()
		for _, teleApp := range teleApps {
			fqdn := AssembleAppFQDN(localClusterName, localProxyDNSName, appClusterName, teleApp)
			labels := []Label{}
			for name, value := range teleApp.StaticLabels {
				labels = append(labels, Label{
					Name:  name,
					Value: value,
				})
			}

			sort.Sort(sortedLabels(labels))

			result = append(result, App{
				Name:       teleApp.Name,
				URI:        teleApp.URI,
				PublicAddr: teleApp.PublicAddr,
				Labels:     labels,
				ClusterID:  appClusterName,
				FQDN:       fqdn,
			})
		}
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
func AssembleAppFQDN(localClusterName string, localProxyDNSName string, appClusterName string, app *types.App) string {
	isLocalCluster := localClusterName == appClusterName
	if isLocalCluster && app.PublicAddr != "" {
		return app.PublicAddr
	}
	return fmt.Sprintf("%v.%v", app.Name, localProxyDNSName)
}
