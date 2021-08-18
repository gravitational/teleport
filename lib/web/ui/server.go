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

	"github.com/gravitational/teleport/api/types"
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
	// Tunnel indicates of this server is connected over a reverse tunnel.
	Tunnel bool `json:"tunnel"`
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

// sortedLabels is a sort wrapper that sorts labels by name
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
func MakeServers(clusterName string, servers []types.Server) []Server {
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
			Labels:      uiLabels,
			Name:        server.GetName(),
			Hostname:    server.GetHostname(),
			Addr:        server.GetAddr(),
			Tunnel:      server.GetUseTunnel(),
		})
	}

	return uiServers
}

// Kube describes a kube cluster.
type Kube struct {
	// Name is the name of the kube cluster.
	Name string `json:"name"`
	// Labels is a map of static and dynamic labels associated with an kube cluster.
	Labels []Label `json:"labels"`
}

// MakeKubes creates ui kube objects and returns a list..
func MakeKubes(clusterName string, servers []types.Server) []Kube {
	kubeClusters := map[string]*types.KubernetesCluster{}

	// Get unique kube clusters
	for _, server := range servers {
		// Process each kube cluster.
		for _, cluster := range server.GetKubernetesClusters() {
			kubeClusters[cluster.Name] = cluster
		}
	}

	uiKubeClusters := make([]Kube, 0, len(kubeClusters))
	for _, cluster := range kubeClusters {
		uiLabels := []Label{}

		for name, value := range cluster.StaticLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: value,
			})
		}

		for name, cmd := range cluster.DynamicLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: cmd.GetResult(),
			})
		}

		sort.Sort(sortedLabels(uiLabels))

		uiKubeClusters = append(uiKubeClusters, Kube{
			Name:   cluster.Name,
			Labels: uiLabels,
		})
	}

	return uiKubeClusters
}

// Database describes a database server.
type Database struct {
	// Name is the name of the database.
	Name string `json:"name"`
	// Desc is the database description.
	Desc string `json:"desc"`
	// Protocol is the database description.
	Protocol string `json:"protocol"`
	// Type is the database type, self-hosted or cloud-hosted.
	Type string `json:"type"`
	// Labels is a map of static and dynamic labels associated with an database.
	Labels []Label `json:"labels"`
}

// MakeDatabases creates database objects.
func MakeDatabases(clusterName string, databases []types.Database) []Database {
	uiServers := make([]Database, 0, len(databases))
	for _, database := range databases {
		uiLabels := []Label{}

		for name, value := range database.GetStaticLabels() {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: value,
			})
		}

		for name, cmd := range database.GetDynamicLabels() {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: cmd.GetResult(),
			})
		}

		sort.Sort(sortedLabels(uiLabels))

		uiServers = append(uiServers, Database{
			Name:     database.GetName(),
			Desc:     database.GetDescription(),
			Protocol: database.GetProtocol(),
			Type:     database.GetType(),
			Labels:   uiLabels,
		})
	}

	return uiServers
}
