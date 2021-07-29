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
