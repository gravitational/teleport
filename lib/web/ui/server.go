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

type sortedLabels []Label

func (s sortedLabels) Len() int {
	return len(s)
}

func (s sortedLabels) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s sortedLabels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// MakeServers creates server objects for webapp
func MakeServers(clusterName string, servers []services.Server) []Server {
	uiServers := []Server{}
	for _, server := range servers {
		uiLabels := []Label{}
		serverLabels := server.GetLabels()
		for name, value := range serverLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: value,
			})
		}

		serverCmdLabels := server.GetCmdLabels()
		for name, cmd := range serverCmdLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: cmd.GetResult(),
			})
		}

		sort.Sort(sortedLabels(uiLabels))

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
