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

// resourceMetadataAdapter is an adapter for RFD 153-style resources.
type resourceMetadataAdapter[T types.ResourceMetadata] struct {
	resource T
}

var _ Resource = resourceMetadataAdapter[types.ResourceMetadata]{}
var _ types.ResourceMetadata = resourceMetadataAdapter[types.ResourceMetadata]{}

func newResourceMetadataAdapter[T types.ResourceMetadata](t T) resourceMetadataAdapter[T] {
	return resourceMetadataAdapter[T]{resource: t}
}

func (w resourceMetadataAdapter[T]) GetMetadata() *headerv1.Metadata {
	return w.resource.GetMetadata()
}

func (w resourceMetadataAdapter[T]) GetName() string {
	return w.resource.GetMetadata().GetName()
}
