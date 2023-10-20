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

	// UpsertClusterAlert creates the specified alert, overwriting any preexising alert with the same ID.
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
