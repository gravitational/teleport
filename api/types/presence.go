/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/trace"
)

// Site represents a cluster of teleport nodes who collectively trust the same
// certificate authority (CA) and have a common name.
//
// The CA is represented by an auth server (or multiple auth servers, if running
// in HA mode)
type Site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"lastconnected"`
	Status        string    `json:"status"`
}

// IsEmpty returns true if keepalive is empty,
// used to indicate that keepalive is not supported
func (s *KeepAlive) IsEmpty() bool {
	return s.LeaseID == 0 && s.Name == ""
}

// GetType return the type of keep alive: either application or server.
func (s *KeepAlive) GetType() string {
	switch s.Type {
	case KeepAlive_NODE:
		return constants.KeepAliveNode
	case KeepAlive_APP:
		return constants.KeepAliveApp
	case KeepAlive_DATABASE:
		return constants.KeepAliveDatabase
	case KeepAlive_WINDOWS_DESKTOP:
		return constants.KeepAliveWindowsDesktopService
	case KeepAlive_KUBERNETES:
		return constants.KeepAliveKube
	default:
		return constants.KeepAliveNode
	}
}

// CheckAndSetDefaults validates this KeepAlive value and sets default values
func (s *KeepAlive) CheckAndSetDefaults() error {
	if s.Namespace == "" {
		s.Namespace = defaults.Namespace
	}
	if s.IsEmpty() {
		return trace.BadParameter("invalid keep alive, missing lease ID and resource name")
	}
	return nil
}

// KeepAliver keeps object alive
type KeepAliver interface {
	// KeepAlives allows to receive keep alives
	KeepAlives() chan<- KeepAlive

	// Done returns the channel signaling the closure
	Done() <-chan struct{}

	// Close closes the watcher and releases
	// all associated resources
	Close() error

	// Error returns error associated with keep aliver if any
	Error() error
}
