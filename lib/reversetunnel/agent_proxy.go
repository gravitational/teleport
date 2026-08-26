/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reversetunnel

import (
	"slices"
	"sync"
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

	if slices.Equal(g.ids, ids) {
		return
	}

	if len(ids) == 0 {
		g.ids = nil
		return
	}

	g.ids = make([]string, len(ids))
	copy(g.ids, ids)
}
