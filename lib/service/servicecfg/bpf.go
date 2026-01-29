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

package servicecfg

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// BPFConfig holds configuration for the BPF service.
type BPFConfig struct {
	// Enabled is if this service will try and install BPF programs on this system.
	Enabled bool

	// CommandBufferSize is the size of the perf buffer for command events.
	CommandBufferSize *int

	// DiskBufferSize is the size of the perf buffer for disk events.
	DiskBufferSize *int

	// NetworkBufferSize is the size of the perf buffer for network events.
	NetworkBufferSize *int
}

// CheckAndSetDefaults checks BPF configuration.
func (c *BPFConfig) CheckAndSetDefaults() error {
	perfBufferPageCount := defaults.PerfBufferPageCount
	openPerfBufferPageCount := defaults.OpenPerfBufferPageCount

	// Set defaults for buffer sizes if they are unset or zero.
	// A zero value was accepted before but is undesirable now as it
	// will result in blocking event channels, so we set it to a sane
	// default to maintain backwards compatibility.
	if c.CommandBufferSize == nil || *c.CommandBufferSize == 0 {
		c.CommandBufferSize = &perfBufferPageCount
	} else if *c.CommandBufferSize < 0 {
		return trace.BadParameter("CommandBufferSize must not be negative")
	}
	if c.DiskBufferSize == nil || *c.DiskBufferSize == 0 {
		c.DiskBufferSize = &openPerfBufferPageCount
	} else if *c.DiskBufferSize < 0 {
		return trace.BadParameter("DiskBufferSize must not be negative")
	}
	if c.NetworkBufferSize == nil || *c.NetworkBufferSize == 0 {
		c.NetworkBufferSize = &perfBufferPageCount
	} else if *c.NetworkBufferSize < 0 {
		return trace.BadParameter("NetworkBufferSize must not be negative")
	}

	return nil
}
