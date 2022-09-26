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

package local

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// StatusService manages cluster status info.
type StatusService struct {
	backend.Backend
	log logrus.FieldLogger
}

func NewStatusService(bk backend.Backend) *StatusService {
	return &StatusService{
		Backend: bk,
		log:     logrus.WithField(trace.Component, "status"),
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
	startKey := backend.Key(clusterAlertPrefix, "")
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

	val, err := utils.FastMarshal(&alert)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Backend.Put(ctx, backend.Item{
		Key:     backend.Key(clusterAlertPrefix, alert.Metadata.Name),
		Value:   val,
		Expires: alert.Metadata.Expiry(),
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

const clusterAlertPrefix = "cluster-alerts"

// Status service manages alerts.
type Status interface {
	GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error)
	UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error
	DeleteClusterAlert(ctx context.Context, alertID string) error
}
