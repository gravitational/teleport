// +build !linux

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

import (
	"github.com/gravitational/teleport/lib/events"
)

// SessionContext contains all the information needed to track and emit
// events for a particular session. Most of this information is already within
// srv.ServerContext, unfortunately due to circular imports with lib/srv and
// lib/bpf, part of that structure is reproduced in SessionContext.
type SessionContext struct {
	// Namespace is the namespace within which this session occurs.
	Namespace string

	// SessionID is the UUID of the given session.
	SessionID string

	// ServerID is the UUID of the server this session is executing on.
	ServerID string

	// Login is the Unix login for this session.
	Login string

	// User is the Teleport user.
	User string

	// PID is the process ID of Teleport when it re-executes itself. This is
	// used by Telepor to find itself by cgroup.
	PID int

	// AuditLog is used to store events for a particular sessionl
	AuditLog events.IAuditLog
}

// Config holds configuration for the BPF service.
type Config struct {
	// Enabled is if this service will try and install BPF programs on this system.
	Enabled bool

	// CgroupMountPath is where the cgroupv2 hierarchy is mounted.
	CgroupMountPath string
}

// Service is used on non-Linux systems as a NOP service that allows the
// caller to open and close sessions that do nothing on systems that don't
// support eBPF.
type Service struct {
}

// New returns a new NOP service. Note this function does nothing.
func New(config *Config) (*Service, error) {
	return &Service{}, nil
}

// Close will close the NOP service. Note this function does nothing.
func (s *Service) Close() error {
	return nil
}

// OpenSession will open a NOP session. Note this function does nothing.
func (s *Service) OpenSession(ctx *SessionContext) error {
	return nil
}

// OpenSession will open a NOP session. Note this function does nothing.
func (s *Service) CloseSession(ctx *SessionContext) error {
	return nil
}
