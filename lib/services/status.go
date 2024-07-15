/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// Status defines an interface for managing cluster status info.
type Status interface {
	// GetClusterAlerts loads all matching cluster alerts.
	GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error)

	// UpsertClusterAlert creates the specified alert, overwriting any preexisting alert with the same ID.
	UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error

	// CreateAlertAck marks a cluster alert as acknowledged.
	CreateAlertAck(ctx context.Context, ack types.AlertAcknowledgement) error

	// GetAlertAcks gets active alert ackowledgements.
	GetAlertAcks(ctx context.Context) ([]types.AlertAcknowledgement, error)

	// ClearAlertAcks clears alert acknowledgments.
	ClearAlertAcks(ctx context.Context, req proto.ClearAlertAcksRequest) error
}

// StatusInternal extends Status with auth-internal methods.
type StatusInternal interface {
	Status

	// DeleteClusterAlert deletes the cluster alert with the specified ID.
	DeleteClusterAlert(ctx context.Context, alertID string) error
}
