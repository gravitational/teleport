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

	"github.com/gravitational/teleport/lib/services"
)

// AppLabel describes application label
type AppLabel struct {
	// Name is this label name
	Name string `json:"name"`
	// Value is this label value
	Value string `json:"value"`
}

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
	// Labels is map of static labels associated with an application.
	Labels []AppLabel `json:"labels"`
}

// MakeApps creates server application objects
func MakeApps(proxyName string, proxyHost string, appClusterName string, appServers []services.Server) []App {
	result := []App{}
	for _, server := range appServers {
		teleApps := server.GetApps()
		for _, teleApp := range teleApps {
			fqdn := resolveFQDN(proxyName, proxyHost, appClusterName, *teleApp)
			labels := []AppLabel{}
			for name, value := range teleApp.StaticLabels {
				labels = append(labels, AppLabel{
					Name:  name,
					Value: value,
				})
			}
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

func resolveFQDN(proxyName string, proxyHost string, appClusterName string, app services.App) string {
	// Use application public address if running on proxy.
	isProxyCluster := proxyName == appClusterName
	if isProxyCluster && app.PublicAddr != "" {
		return app.PublicAddr
	}

	if proxyHost != "" {
		return fmt.Sprintf("%v.%v", app.Name, proxyHost)
	}

	return fmt.Sprintf("%v.%v", app.Name, appClusterName)
}
