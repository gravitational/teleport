/*
Copyright 2022 Gravitational, Inc.

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

package reversetunnel

import (
	"sync"

	"github.com/google/go-cmp/cmp"
)

// NewConnectedProxyGetter creates a new ConnectedProxyGetter instance.
func NewConnectedProxyGetter() *ConnectedProxyGetter {
	return &ConnectedProxyGetter{}
}

// ConnectedProxyGetter gets the proxy ids that the a reverse tunnel pool is
// connected to. This is used to communicate the connected proxies between
// reversetunnel.AgentPool and service implementations to include the
// connected proxy ids in the service's heartbeats.
type ConnectedProxyGetter struct {
	ids []string
	mu  sync.RWMutex
}

// GetProxyIDs gets the list of connected proxy ids.
func (g *ConnectedProxyGetter) GetProxyIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.ids
}

// setProxyIDs sets the list of connected proxy ids.
func (g *ConnectedProxyGetter) setProxyIDs(ids []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cmp.Equal(g.ids, ids) {
		return
	}

	if len(ids) == 0 {
		g.ids = nil
		return
	}

	g.ids = make([]string, len(ids))
	copy(g.ids, ids)
}
