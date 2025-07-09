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

//go:build vnetdaemon
// +build vnetdaemon

package vnet

import (
	"context"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	vnetdaemon "github.com/gravitational/teleport/lib/vnet/daemon"
)

func (s *Service) GetBackgroundItemStatus(ctx context.Context, req *api.GetBackgroundItemStatusRequest) (*api.GetBackgroundItemStatusResponse, error) {
	status, err := vnetdaemon.DaemonStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetBackgroundItemStatusResponse{
		Status: backgroundItemStatusFromServiceStatus(status),
	}, nil
}

func backgroundItemStatusFromServiceStatus(status vnetdaemon.ServiceStatus) api.BackgroundItemStatus {
	switch status {
	case vnetdaemon.ServiceStatusNotRegistered:
		return api.BackgroundItemStatus_BACKGROUND_ITEM_STATUS_NOT_REGISTERED
	case vnetdaemon.ServiceStatusEnabled:
		return api.BackgroundItemStatus_BACKGROUND_ITEM_STATUS_ENABLED
	case vnetdaemon.ServiceStatusRequiresApproval:
		return api.BackgroundItemStatus_BACKGROUND_ITEM_STATUS_REQUIRES_APPROVAL
	case vnetdaemon.ServiceStatusNotFound:
		return api.BackgroundItemStatus_BACKGROUND_ITEM_STATUS_NOT_FOUND
	case vnetdaemon.ServiceStatusNotSupported:
		return api.BackgroundItemStatus_BACKGROUND_ITEM_STATUS_NOT_SUPPORTED
	default:
		return api.BackgroundItemStatus_BACKGROUND_ITEM_STATUS_UNSPECIFIED
	}
}
