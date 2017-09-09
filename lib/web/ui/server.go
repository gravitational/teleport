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
	"sort"

	"github.com/gravitational/teleport/lib/services"
)

// Label describes label for webapp
type Label struct {
	// Name is this label name
	Name string `json:"name"`
	// Value is this label value
	Value string `json:"value"`
}

// Server describes a server for webapp
type Server struct {
	// Name is this server name
	Name string `json:"id"`
	// ClusterName is this server cluster name
	ClusterName string `json:"siteId"`
	// Hostname is this server hostname
	Hostname string `json:"hostname"`
	// Addrr is this server ip address
	Addr string `json:"addr"`
	// Labels is this server list of labels
	Labels []Label `json:"tags"`
}

// MakeServers creates server objects for webapp
func MakeServers(clusterName string, servers []services.Server) []Server {
	uiServers := []Server{}
	for _, server := range servers {
		serverLabels := server.GetLabels()
		labelNames := []string{}
		for name := range serverLabels {
			labelNames = append(labelNames, name)
		}

		// sort labels by name
		sort.Strings(labelNames)
		uiLabels := []Label{}
		for _, name := range labelNames {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: serverLabels[name],
			})
		}

		uiServers = append(uiServers, Server{
			ClusterName: clusterName,
			Name:        server.GetName(),
			Hostname:    server.GetHostname(),
			Addr:        server.GetAddr(),
			Labels:      uiLabels,
		})
	}

	return uiServers
}
