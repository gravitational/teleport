/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

// SessionContext contains common context parameters for an App session.
type SessionContext struct {
	// Identity is the requested identity.
	Identity *tlsca.Identity
	// App is the requested identity.
	App types.Application
	// ChunkID is the session chunk's uuid.
	ChunkID string
	// Audit is used to emit audit events for the session.
	Audit Audit
}

// Check validates the SessionContext.
func (sc *SessionContext) Check() error {
	if sc.Identity == nil {
		return trace.BadParameter("missing Identity")
	}
	if sc.App == nil {
		return trace.BadParameter("missing App")
	}
	if sc.ChunkID == "" {
		return trace.BadParameter("missing ChunkID")
	}
	if sc.Audit == nil {
		return trace.BadParameter("missing Audit")
	}
	return nil
}
