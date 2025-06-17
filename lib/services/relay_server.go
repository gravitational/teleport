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
	"github.com/gravitational/teleport/api/types/common"
)

// ValidateRelayServer will check the given relay server for validity. Should be
// called before writing a new value in the cluster state storage and before
// using a value. The value will not be modified.
func ValidateRelayServer(resource *presencev1.RelayServer) error {
	if expected, actual := apitypes.KindRelayServer, resource.GetKind(); expected != actual {
		return trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := "", resource.GetSubKind(); expected != actual {
		return trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if expected, actual := apitypes.V1, resource.GetVersion(); expected != actual {
		return trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if name := resource.GetMetadata().GetName(); name == "" {
		return trace.BadParameter("missing name")
	}
	for key := range resource.GetMetadata().GetLabels() {
		if key == apitypes.OriginLabel {
			return trace.BadParameter("origin label unsupported")
		}
		if !common.IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key %q", key)
		}
	}

	// TODO(espadolini): validate spec contents, nothing to validate so far

	return nil
}
