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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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
}

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentProxy,
})

// NewClusters creates a slice of Cluster's, containing data about each cluster.
func NewClusters(remoteClusters []reversetunnel.RemoteSite) ([]Cluster, error) {
	clusters := []Cluster{}
	for _, rclsr := range remoteClusters {
		clt, err := rclsr.CachingAccessPoint()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		
		// error is not handled b/c len(nil) is 0
		nodes, _ := clt.GetNodes(defaults.Namespace)

		proxies, err := clt.GetProxies()
		if err != nil {
			log.Errorf("Unable to retrieve proxy list: %v", err)
			proxies = []services.Server{}
		}
		
		proxyHost := services.GuessProxyHost(proxies)

		authServers, err := clt.GetAuthServers()
		if err != nil {
			log.Errorf("Unable to retrieve auth servers list: %v", err)
		}
		
		authVersion := ""
		if len(authServers) > 0 {
			// use the first auth server
			authVersion = authServers[0].GetTeleportVersion()
		}

		clusters = append(clusters, Cluster{
			Name:          rclsr.GetName(),
			LastConnected: rclsr.GetLastConnected(),
			Status:        rclsr.GetStatus(),
			NodeCount:     len(nodes),
			PublicURL:     proxyHost,
			AuthVersion:   authVersion,
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	return clusters, nil
}
