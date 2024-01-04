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

package restrictedsession

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/services"
)

// RestrictionsWatcherClient is used by changeset to fetch a list
// of proxies and subscribe to updates
type RestrictionsWatcherClient interface {
	services.Restrictions
	types.Events
}

// Manager starts and stop enforcing restrictions for a given session.
type Manager interface {
	// OpenSession starts enforcing restrictions for a cgroup with cgroupID
	OpenSession(ctx *bpf.SessionContext, cgroupID uint64)
	// CloseSession stops enforcing restrictions for a cgroup with cgroupID
	CloseSession(ctx *bpf.SessionContext, cgroupID uint64)
	// Close stops the manager, cleaning up any resources
	Close()
}

// Stubbed out Manager interface for cases where the real thing is not used.
type NOP struct{}

func (NOP) OpenSession(ctx *bpf.SessionContext, cgroupID uint64) {
}

func (NOP) CloseSession(ctx *bpf.SessionContext, cgroupID uint64) {
}

func (NOP) UpdateNetworkRestrictions(r *NetworkRestrictions) error {
	return nil
}

func (NOP) Close() {
}
