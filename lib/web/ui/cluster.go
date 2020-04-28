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
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Cluster describes a cluster
type Cluster struct {
	// Name is the cluster name
	Name string `json:"name"`
	// LastConnected is the cluster last connected time
	LastConnected time.Time `json:"lastConnected"`
	// Status is the cluster status
	Status string `json:"status"`
	// NodeCount is this cluster number of registered servers
	NodeCount int `json:"nodeCount"`
	// PublicURL is this cluster public URL (its first available proxy URL)
	PublicURL string `json:"publicURL"`
	// AuthVersion is the cluster auth's service version
	AuthVersion string `json:"authVersion"`
	// ProxyVersion is the cluster proxy's service version
	ProxyVersion string `json:"proxyVersion"`
}

// NewClusters creates a slice of Cluster's, containing data about each cluster.
func NewClusters(remoteClusters []reversetunnel.RemoteSite) ([]Cluster, error) {
	clusters := []Cluster{}
	for _, site := range remoteClusters {
		cluster, err := GetClusterDetails(site)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusters = append(clusters, *cluster)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	return clusters, nil
}

// GetClusterDetails retrieves and sets details about a cluster
func GetClusterDetails(site reversetunnel.RemoteSite) (*Cluster, error) {
	clt, err := site.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nodes, err := clt.GetNodes(defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxies, err := clt.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyHost, proxyVersion := services.GuessProxyHostAndVersion(proxies)

	authServers, err := clt.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authVersion := ""
	if len(authServers) > 0 {
		// use the first auth server
		authVersion = authServers[0].GetTeleportVersion()
	}

	return &Cluster{
		Name:          site.GetName(),
		LastConnected: site.GetLastConnected(),
		Status:        site.GetStatus(),
		NodeCount:     len(nodes),
		PublicURL:     proxyHost,
		AuthVersion:   authVersion,
		ProxyVersion:  proxyVersion,
	}, nil
}
