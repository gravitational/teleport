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

package local

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// StatusService manages cluster status info.
type StatusService struct {
	backend.Backend
	log logrus.FieldLogger
}

func NewStatusService(bk backend.Backend) *StatusService {
	return &StatusService{
		Backend: bk,
		log:     logrus.WithField(teleport.ComponentKey, "status"),
	}
}

func (s *StatusService) GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	var alerts []types.ClusterAlert
	if query.AlertID != "" {
		alert, err := s.getClusterAlert(ctx, query.AlertID)
		if err != nil {
			if trace.IsNotFound(err) {
				// return an empty list
				return nil, nil
			}
			return nil, trace.Wrap(err)
		}
		alerts = []types.ClusterAlert{alert}
	} else {
		var err error
		alerts, err = s.getAllClusterAlerts(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	filtered := alerts[:0]
	for _, alert := range alerts {
		if err := alert.CheckAndSetDefaults(); err != nil {
			s.log.Warnf("Skipping invalid cluster alert: %v", err)
		}

		if !query.Match(alert) {
			continue
		}

		filtered = append(filtered, alert)
	}

	return filtered, nil
}

func (s *StatusService) getAllClusterAlerts(ctx context.Context) ([]types.ClusterAlert, error) {
	startKey := backend.ExactKey(clusterAlertPrefix)
	result, err := s.Backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	alerts := make([]types.ClusterAlert, 0, len(result.Items))

	for _, item := range result.Items {
		var alert types.ClusterAlert
		if err := utils.FastUnmarshal(item.Value, &alert); err != nil {
			return nil, trace.Wrap(err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (s *StatusService) getClusterAlert(ctx context.Context, alertID string) (types.ClusterAlert, error) {
	key := backend.Key(clusterAlertPrefix, alertID)
	item, err := s.Backend.Get(ctx, key)
	if err != nil {
		return types.ClusterAlert{}, trace.Wrap(err)
	}

	var alert types.ClusterAlert
	if err := utils.FastUnmarshal(item.Value, &alert); err != nil {
		return types.ClusterAlert{}, trace.Wrap(err)
	}

	return alert, nil
}

func (s *StatusService) UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error {
	if err := alert.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if alert.Spec.Created.IsZero() {
		alert.Spec.Created = s.Clock().Now().UTC()
	}

	if alert.Metadata.Expiry().IsZero() {
		alert.Metadata.SetExpiry(alert.Spec.Created.Add(time.Hour * 24))
	}

	rev := alert.GetRevision()
	val, err := utils.FastMarshal(&alert)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Backend.Put(ctx, backend.Item{
		Key:      backend.Key(clusterAlertPrefix, alert.Metadata.Name),
		Value:    val,
		Expires:  alert.Metadata.Expiry(),
		Revision: rev,
	})
	return trace.Wrap(err)
}

func (s *StatusService) DeleteClusterAlert(ctx context.Context, alertID string) error {
	err := s.Backend.Delete(ctx, backend.Key(clusterAlertPrefix, alertID))
	if trace.IsNotFound(err) {
		return trace.NotFound("cluster alert %q not found", alertID)
	}
	return trace.Wrap(err)
}

// CreateAlertAck marks a cluster alert as acknowledged.
func (s *StatusService) CreateAlertAck(ctx context.Context, ack types.AlertAcknowledgement) error {
	if err := ack.Check(); err != nil {
		return trace.Wrap(err)
	}

	val, err := utils.FastMarshal(&ack)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Backend.Create(ctx, backend.Item{
		Key:     backend.Key(alertAckPrefix, ack.AlertID),
		Value:   val,
		Expires: ack.Expires,
	})
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("alert %q has already been acknowledged", ack.AlertID)
	}
	return trace.Wrap(err)
}

// GetAlertAcks gets active alert ackowledgements.
func (s *StatusService) GetAlertAcks(ctx context.Context) ([]types.AlertAcknowledgement, error) {
	startKey := backend.ExactKey(alertAckPrefix)
	result, err := s.Backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	acks := make([]types.AlertAcknowledgement, 0, len(result.Items))

	for _, item := range result.Items {
		var ack types.AlertAcknowledgement
		if err := utils.FastUnmarshal(item.Value, &ack); err != nil {
			return nil, trace.Wrap(err)
		}
		acks = append(acks, ack)
	}

	return acks, nil
}

// ClearAlertAcks clears alert acknowledgments.
func (s *StatusService) ClearAlertAcks(ctx context.Context, req proto.ClearAlertAcksRequest) error {
	if req.AlertID == "" {
		return trace.BadParameter("missing alert id for ack clear")
	}
	if req.AlertID == types.Wildcard {
		startKey := backend.ExactKey(alertAckPrefix)
		return trace.Wrap(s.Backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
	}

	err := s.Backend.Delete(ctx, backend.Key(alertAckPrefix, req.AlertID))
	if trace.IsNotFound(err) {
		return nil
	}
	return trace.Wrap(err)
}

const clusterAlertPrefix = "cluster-alerts"

const alertAckPrefix = "alert-ack"
