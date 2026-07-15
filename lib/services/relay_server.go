// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"github.com/gravitational/trace"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	apitypes "github.com/gravitational/teleport/api/types"
)

// ValidateRelayServer will check the given relay server for validity. Should be
// called before writing a new value in the cluster state storage and before
// using a value. The value will not be modified.
func ValidateRelayServer(resource *presencev1.RelayServer) error {
	if err := validateHostHeartbeatEnvelope(resource, apitypes.KindRelayServer); err != nil {
		return trace.Wrap(err)
	}

	// TODO(espadolini): validate spec contents, nothing to validate so far

	return nil
}
