// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package generic

import (
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// resourceMetadataAdapter is an adapter for RFD 153-style resources. For inner types T which implement
// types.ResourceMetadata, the type resourceMetadataAdapter will implement both types.ResourceMetadata and Resource.
//
// resourceMetadataAdapter is similar to resource153ToLegacyAdapter which can be found api/types/resource_153.go,
// yet resource153ToLegacyAdapter is different in several ways:
// - it isn't generic,
// - it is unexported,
// - it implements the entire types.Resource interface,
// - it makes certain opinionated implementation choices, including calling panic().
type resourceMetadataAdapter[T types.ResourceMetadata] struct {
	resource T
}

func newResourceMetadataAdapter[T types.ResourceMetadata](t T) resourceMetadataAdapter[T] {
	return resourceMetadataAdapter[T]{resource: t}
}

// GetMetadata returns inner metadata from type. Required for dynamically typed helpers like types.GetRevision.
func (w resourceMetadataAdapter[T]) GetMetadata() *headerv1.Metadata {
	return w.resource.GetMetadata()
}

// GetName returns name. Required for Resource.
func (w resourceMetadataAdapter[T]) GetName() string {
	return w.resource.GetMetadata().GetName()
}
