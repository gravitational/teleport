// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicecfg

import (
	"time"

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

	// CgroupPath is where the cgroupv2 hierarchy is mounted.
	CgroupPath string

	// UDP enables UDP connection events.
	UDP bool

	// UDPSilencePeriod is the period in which subsequent UDP sends are silenced
	// to avoid audit noise.
	//
	// Set to zero to log every send.
	//
	// Defaults to [defaults.UDPSilencePeriod].
	UDPSilencePeriod *time.Duration

	// UDPSilenceBufferSize is the max number of concurrently silenced UDP
	// sockets.
	//
	// See [defaults.UDPSilenceBufferSize].
	UDPSilenceBufferSize *int
}

// CheckAndSetDefaults checks BPF configuration.
func (c *BPFConfig) CheckAndSetDefaults() error {
	var perfBufferPageCount = defaults.PerfBufferPageCount
	var openPerfBufferPageCount = defaults.OpenPerfBufferPageCount

	if c.CommandBufferSize == nil {
		c.CommandBufferSize = &perfBufferPageCount
	}
	if c.DiskBufferSize == nil {
		c.DiskBufferSize = &openPerfBufferPageCount
	}
	if c.NetworkBufferSize == nil {
		c.NetworkBufferSize = &perfBufferPageCount
	}
	if c.CgroupPath == "" {
		c.CgroupPath = defaults.CgroupPath
	}
	if c.UDPSilencePeriod == nil {
		val := defaults.UDPSilencePeriod
		c.UDPSilencePeriod = &val
	}
	if c.UDPSilenceBufferSize == nil {
		val := defaults.UDPSilenceBufferSize
		c.UDPSilenceBufferSize = &val
	}

	return nil
}

// RestrictedSessionConfig holds configuration for the RestrictedSession service.
type RestrictedSessionConfig struct {
	// Enabled if this service will try and install BPF programs on this system.
	Enabled bool

	// EventsBufferSize is the size (in pages) of the perf buffer for events.
	EventsBufferSize *int
}

// CheckAndSetDefaults checks BPF configuration.
func (c *RestrictedSessionConfig) CheckAndSetDefaults() error {
	var perfBufferPageCount = defaults.PerfBufferPageCount

	if c.EventsBufferSize == nil {
		c.EventsBufferSize = &perfBufferPageCount
	}

	return nil
}
