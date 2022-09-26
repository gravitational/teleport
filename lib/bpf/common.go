/*
Copyright 2019 Gravitational, Inc.

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

package bpf

// #cgo LDFLAGS: -ldl
// #include <dlfcn.h>
// #include <stdlib.h>
import "C"

import (
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

// BPF implements an interface to open and close a recording session.
type BPF interface {
	// OpenSession will start monitoring all events within a session and
	// emitting them to the Audit Log.
	OpenSession(ctx *srv.ServerContext) (uint64, error)

	// CloseSession will stop monitoring events for a particular session.
	CloseSession(ctx *srv.ServerContext) error

	// Close will stop any running BPF programs.
	Close() error
}

// Config holds configuration for the BPF service.
type Config struct {
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
}

// CheckAndSetDefaults checks BPF configuration.
func (c *Config) CheckAndSetDefaults() error {
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

	return nil
}

// NOP is used on either non-Linux systems or when BPF support is not enabled.
type NOP struct{}

// Close closes the NOP service. Note this function does nothing.
func (s *NOP) Close() error {
	return nil
}

// OpenSession opens a NOP session. Note this function does nothing.
func (s *NOP) OpenSession(_ *srv.ServerContext) (uint64, error) {
	return 0, nil
}

// CloseSession closes a NOP session. Note this function does nothing.
func (s *NOP) CloseSession(_ *srv.ServerContext) error {
	return nil
}

// IsHostCompatible checks that BPF programs can run on this host.
func IsHostCompatible() error {
	minKernel := semver.New(constants.EnhancedRecordingMinKernel)
	version, err := utils.KernelVersion()
	if err != nil {
		return trace.Wrap(err)
	}
	if version.LessThan(*minKernel) {
		return trace.BadParameter("incompatible kernel found, minimum supported kernel is %v", minKernel)
	}

	if err = utils.HasBTF(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
