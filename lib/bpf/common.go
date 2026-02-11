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

package bpf

import (
	"context"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/utils"
)

// BPF implements an interface to open and close a recording session.
type BPF interface {
	// OpenSession will start monitoring all events within a session and
	// emitting them to the Audit Log.
	OpenSession(ctx *SessionContext) error

	// CloseSession will stop monitoring events for a particular session.
	CloseSession(ctx *SessionContext) error

	// Close will stop any running BPF programs.
	Close(restarting bool) error

	// Enabled returns whether enhanced recording is active.
	Enabled() bool
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

	// UserOriginClusterName is the name of the cluster where the user is
	// originally from.
	UserOriginClusterName string

	// Emitter is used to record events for a particular session
	Emitter apievents.Emitter

	// Events is the set of events (command, disk, or network) to record for
	// this session.
	Events map[string]struct{}

	// UserRoles are the roles assigned to the user.
	UserRoles []string
	// UserTraits are the traits assigned to the user.
	UserTraits wrappers.Traits

	// AuditSessionID is the audit session ID that should be the same
	// for all processes in an SSH session. It is used to correlate
	// audit events with the session. See
	// https://github.com/torvalds/linux/blob/b75d8f38bcc9599af42635530c00268c71911f11/Documentation/ABI/stable/procfs-audit_loginuid
	// for more details.
	AuditSessionID uint32
}

// NOP is used on either non-Linux systems or when BPF support is not enabled.
type NOP struct{}

// Close closes the NOP service. Note this function does nothing.
func (s *NOP) Close(_ bool) error {
	return nil
}

// OpenSession opens a NOP session. Note this function does nothing.
func (s *NOP) OpenSession(_ *SessionContext) error {
	return nil
}

// CloseSession closes a NOP session. Note this function does nothing.
func (s *NOP) CloseSession(_ *SessionContext) error {
	return nil
}

func (s *NOP) Enabled() bool {
	return false
}

// IsHostCompatible checks that BPF programs can run on this host.
func IsHostCompatible() error {
	version, err := utils.KernelVersion()
	if err != nil {
		return trace.Wrap(err)
	}
	minKernelVersion := semver.Version{Major: 5, Minor: 8, Patch: 0}
	if version.LessThan(minKernelVersion) {
		return trace.BadParameter("incompatible kernel found, minimum supported kernel is %v", minKernelVersion)
	}

	if err = utils.HasBTF(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
