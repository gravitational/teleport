/*
Copyright 2021 Gravitational, Inc.

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

package implementation

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/nodetracker/api"

	"github.com/jonboulle/clockwork"
)

// tracker is a node tracker database
// This might be moved to teleport/e or somewhere else private
type tracker struct {
	route *routeDetails
	done  chan struct{}
	clock clockwork.Clock
}

type Config struct {
	// Clock is used to control proxy expiry time.
	Clock clockwork.Clock

	// OfflineThreshold is used to set proxy expiry time
	OfflineThreshold time.Duration

	// ProxyControlCallback provides a callback method for proxy cleanup
	// The only current usage for it is in tests. check tracker_test.go
	ProxyControlCallback func()
}

// CheckAndSetDefaults validates the values of a *Config.
func (c *Config) CheckAndSetDefaults() {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.OfflineThreshold == 0 {
		c.OfflineThreshold = defaults.KeepAliveInterval()
	}

	if c.ProxyControlCallback == nil {
		c.ProxyControlCallback = func() {}
	}
}

// NewTracker returns a tracker responsible of tracking node to proxy relationships
func NewTracker(config *Config) api.Tracker {
	config.CheckAndSetDefaults()

	t := &tracker{
		route: newRouteDetails(config.Clock),
		done:  make(chan struct{}),
		clock: config.Clock,
	}

	cleanupTicker := config.Clock.NewTicker(config.OfflineThreshold)
	go func() {
		for {
			select {
			case <-t.done:
				return
			case <-cleanupTicker.Chan():
				t.route.Cleanup(config.OfflineThreshold)
				config.ProxyControlCallback()
			}
		}
	}()

	return t
}

// Stop provides an exit for goroutines spawn by the tracker
func (t *tracker) Stop() {
	t.done <- struct{}{}
	close(t.done)
}

// AddNode stores a new node to proxy relationship
func (t *tracker) AddNode(
	ctx context.Context,
	nodeID string,
	proxyID string,
	clusterName string,
	addr string,
) {
	t.route.Add(
		nodeID,
		api.ProxyDetails{
			ID:          proxyID,
			ClusterName: clusterName,
			Addr:        addr,
			UpdatedAt:   t.clock.Now(),
		},
	)
}

// RemoveNode deletes a node to proxy relationship
func (t *tracker) RemoveNode(ctx context.Context, nodeID string) {
	t.route.RemoveNode(nodeID)
}

// GetNode returns a new node to proxy relationship
func (t *tracker) GetProxies(ctx context.Context, nodeID string) []api.ProxyDetails {
	return t.route.GetProxies(nodeID)
}

// routeDetails is a struct containing node to proxy routes and
// details about corresponding proxies
type routeDetails struct {
	sync.RWMutex
	clock   clockwork.Clock
	route   map[string]map[string]struct{} // key NodeID, value set of ProxyID
	details map[string]api.ProxyDetails    // key ProxyID, value ProxyDetails
}

// newRouteDetails inits a new routeDetails struct
func newRouteDetails(clock clockwork.Clock) *routeDetails {
	rd := &routeDetails{clock: clock}
	rd.route = make(map[string]map[string]struct{})
	rd.details = make(map[string]api.ProxyDetails)
	return rd
}

// Cleanup removes expired proxies and their node relations from the tracker
func (rd *routeDetails) Cleanup(offlineThreshold time.Duration) {
	rd.Lock()
	defer rd.Unlock()

	// remove expired proxies
	removedProxies := make(map[string]struct{})
	for proxyID, proxyDetail := range rd.details {
		if rd.clock.Since(proxyDetail.UpdatedAt) > offlineThreshold {
			delete(rd.details, proxyID)
			removedProxies[proxyID] = struct{}{}
		}
	}

	// remove expired node to proxy relationships
	for nodeID, proxySet := range rd.route {
		for proxyID := range proxySet {
			if _, ok := removedProxies[proxyID]; ok {
				delete(proxySet, proxyID)
			}
		}
		if len(proxySet) == 0 {
			delete(rd.route, nodeID)
		}
	}
}

// Add adds a new node to proxy relationship
func (rd *routeDetails) Add(nodeID string, proxyDetails api.ProxyDetails) {
	rd.Lock()
	defer rd.Unlock()

	if _, ok := rd.route[nodeID]; !ok {
		rd.route[nodeID] = make(map[string]struct{})
	}

	rd.route[nodeID][proxyDetails.ID] = struct{}{}
	rd.details[proxyDetails.ID] = proxyDetails
}

// GetProxies returns an ordered list of proxies related to nodeID
func (rd *routeDetails) GetProxies(nodeID string) []api.ProxyDetails {
	rd.RLock()
	defer rd.RUnlock()

	proxyDetails := make([]api.ProxyDetails, 0)
	for proxyID := range rd.route[nodeID] {
		if proxyDetail, ok := rd.details[proxyID]; ok {
			proxyDetails = append(proxyDetails, proxyDetail)
		}
	}

	sort.Slice(proxyDetails, func(i, j int) bool {
		return proxyDetails[i].UpdatedAt.After(proxyDetails[j].UpdatedAt)
	})

	return proxyDetails
}

// RemoveNode removes a node to proxy relationship
func (rd *routeDetails) RemoveNode(nodeID string) {
	rd.Lock()
	defer rd.Unlock()

	delete(rd.route, nodeID)
}

// RemoveProxy removes a proxy and it's corresponding details and node relationships
func (rd *routeDetails) RemoveProxy(proxyID string) {
	rd.Lock()
	defer rd.Unlock()

	for _, proxySet := range rd.route {
		delete(proxySet, proxyID)
	}

	delete(rd.details, proxyID)
}
