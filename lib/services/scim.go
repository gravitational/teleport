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

package services

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

// SCIM is an internal abstraction for the SCIM provisioning service, allowing clients
// running over GRPC and local clients to interact with the service in the same way.
type SCIM interface {
	ListSCIMResources(context.Context, *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error)
	GetSCIMResource(context.Context, *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error)
	UpdateSCIMResource(context.Context, *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error)
	CreateSCIMResource(context.Context, *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error)
	DeleteSCIMResource(context.Context, *scimpb.DeleteSCIMResourceRequest) (*emptypb.Empty, error)
}
