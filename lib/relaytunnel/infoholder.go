// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package relaytunnel

import (
	"slices"
	"sync"
)

type GetRelayInfoFunc = func() (relayGroup string, relayIDs []string)
type SetRelayInfoFunc = func(relayGroup string, relayIDs []string)

// InfoHolder is a concurrency-safe holder for the data from a relay tunnel
// client that should be advertised in server heartbeats, i.e. the relay group
// name and the list of relay host IDs that the client is connected to.
type InfoHolder struct {
	mu sync.Mutex

	relayGroup string
	relayIDs   []string
}

// GetRelayInfo returns a copy of the relay client info. The relay group name
// can be the empty string and the list of relay host IDs can be empty.
func (r *InfoHolder) GetRelayInfo() (relayGroup string, relayIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.relayGroup, slices.Clone(r.relayIDs)
}

var _ GetRelayInfoFunc = (*InfoHolder)(nil).GetRelayInfo

// SetRelayInfo stores the given relay client info in the holder, to be later
// given out via [InfoHolder.GetRelayInfo].
func (r *InfoHolder) SetRelayInfo(relayGroup string, relayIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.relayGroup = relayGroup
	r.relayIDs = relayIDs
}

var _ SetRelayInfoFunc = (*InfoHolder)(nil).SetRelayInfo
