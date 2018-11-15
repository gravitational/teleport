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

	"github.com/gravitational/teleport/lib/reversetunnel"
)

// Cluster describes a cluster
type Cluster struct {
	// Name is the cluster name
	Name string `json:"name"`
	// LastConnected is the cluster last connected time
	LastConnected time.Time `json:"last_connected"`
	// Status is the cluster status
	Status string `json:"status"`
}

// AvailableClusters describes all available clusters
type AvailableClusters struct {
	// Current describes current cluster
	Current Cluster `json:"current"`
	// Trusted describes trusted clusters
	Trusted []Cluster `json:"trusted"`
}

// NewAvailableClusters returns all available clusters
func NewAvailableClusters(currentClusterName string, remoteClusters []reversetunnel.RemoteSite) *AvailableClusters {
	out := AvailableClusters{}
	for _, item := range remoteClusters {
		cluster := Cluster{
			Name:          item.GetName(),
			LastConnected: item.GetLastConnected(),
			Status:        item.GetStatus(),
		}

		if item.GetName() == currentClusterName {
			out.Current = cluster
		} else {
			out.Trusted = append(out.Trusted, cluster)
		}
	}

	sort.Slice(out.Trusted, func(i, j int) bool {
		return out.Trusted[i].Name < out.Trusted[j].Name
	})

	return &out
}
