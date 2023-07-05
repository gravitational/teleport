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

import "C"

import (
	"context"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// BPF implements an interface to open and close a recording session.
type BPF interface {
	// OpenSession will start monitoring all events within a session and
	// emitting them to the Audit Log.
	OpenSession(ctx *SessionContext) (uint64, error)

	// CloseSession will stop monitoring events for a particular session.
	CloseSession(ctx *SessionContext) error

	// Close will stop any running BPF programs.
	Close(restarting bool) error
}

// SessionContext contains all the information needed to track and emit
// events for a particular session. Most of this information is already within
// srv.ServerContext, unfortunately due to circular imports with lib/srv and
// lib/bpf, part of that structure is reproduced in SessionContext.
type SessionContext struct {
	// Context is a cancel context, scoped to a server, and not a session.
	Context context.Context

	// Namespace is the namespace within which this session occurs.
	Namespace string

	// SessionID is the UUID of the given session.
	SessionID string

	// ServerID is the UUID of the server this session is executing on.
	ServerID string

	// ServerHostname is the hostname of the server this session is executing on.
	ServerHostname string

	// Login is the Unix login for this session.
	Login string

	// User is the Teleport user.
	User string

	// PID is the process ID of Teleport when it re-executes itself. This is
	// used by Teleport to find itself by cgroup.
	PID int

	// Emitter is used to record events for a particular session
	Emitter apievents.Emitter

	// Events is the set of events (command, disk, or network) to record for
	// this session.
	Events map[string]bool
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
type NOP struct {
}

// Close closes the NOP service. Note this function does nothing.
func (s *NOP) Close(bool) error {
	return nil
}

// OpenSession opens a NOP session. Note this function does nothing.
func (s *NOP) OpenSession(_ *SessionContext) (uint64, error) {
	return 0, nil
}

// CloseSession closes a NOP session. Note this function does nothing.
func (s *NOP) CloseSession(_ *SessionContext) error {
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
