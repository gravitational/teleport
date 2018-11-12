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

	"github.com/gravitational/trace"
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
func NewAvailableClusters(rs []reversetunnel.RemoteSite) (*AvailableClusters, error) {
	if len(rs) == 0 {
		return nil, trace.BadParameter("missing parameter remote clusters")
	}

	// current cluster is the first element in rs[]
	current := Cluster{
		Name:          rs[0].GetName(),
		LastConnected: rs[0].GetLastConnected(),
		Status:        rs[0].GetStatus(),
	}

	// now go through trusted clusters
	trusted := make([]Cluster, len(rs)-1)
	for i := 1; i < len(rs); i++ {
		trusted[i-1] = Cluster{
			Name:          rs[i].GetName(),
			LastConnected: rs[i].GetLastConnected(),
			Status:        rs[i].GetStatus(),
		}
	}

	sort.Sort(byClusterName(trusted))

	return &AvailableClusters{
		Current: current,
		Trusted: trusted,
	}, nil
}

type byClusterName []Cluster

func (s byClusterName) Len() int {
	return len(s)
}

func (s byClusterName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s byClusterName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
