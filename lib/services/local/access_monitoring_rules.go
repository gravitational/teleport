/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accessmonitoringrule"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	accessMonitoringRulesPrefix = "access_monitoring_rule"
)

var _ services.AccessMonitoringRules = (*AccessMonitoringRulesService)(nil)

// AccessMonitoringRulesService manages AccessMonitoringRules in the Backend.
type AccessMonitoringRulesService struct {
	svc generic.Service[*accessmonitoringrule.AccessMonitoringRule]
}

// NewAccessMonitoringRulesService creates a new AccessMonitoringRulesService.
func NewAccessMonitoringRulesService(backend backend.Backend) (*AccessMonitoringRulesService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[*accessmonitoringrule.AccessMonitoringRule]{
		Backend:       backend,
		PageLimit:     defaults.MaxIterationLimit,
		ResourceKind:  types.KindAccessMonitoringRule,
		BackendPrefix: accessMonitoringRulesPrefix,
		MarshalFunc:   services.MarshalAccessMonitoringRule,
		UnmarshalFunc: services.UnmarshalAccessMonitoringRule,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessMonitoringRulesService{
		svc: *svc,
	}, nil
}

// ListAccessMonitoringRules returns a paginated list of AccessMonitoringRule resources.
func (s *AccessMonitoringRulesService) ListAccessMonitoringRules(ctx context.Context, pageSize int, pageToken string) ([]*accessmonitoringrule.AccessMonitoringRule, string, error) {
	igs, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return igs, nextKey, nil
}

// GetAccessMonitoringRule returns the specified AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrule.AccessMonitoringRule, error) {
	ig, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ig, nil
}

// CreateAccessMonitoringRule creates a new AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) CreateAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error) {
	if err := services.CheckAndSetDefaults(amr); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.svc.CreateResource(ctx, amr)
	return created, trace.Wrap(err)
}

// UpdateAccessMonitoringRule updates an existing AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) UpdateAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error) {
	if err := services.CheckAndSetDefaults(amr); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.svc.UpdateResource(ctx, amr)
	return updated, trace.Wrap(err)
}

// UpsertAccessMonitoringRule upserts an existing AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) UpsertAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error) {
	if err := services.CheckAndSetDefaults(amr); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.svc.UpsertResource(ctx, amr)
	return upserted, trace.Wrap(err)
}

// DeleteAccessMonitoringRule removes the specified AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) DeleteAccessMonitoringRule(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllAccessMonitoringRules removes all AccessMonitoringRule resources.
func (s *AccessMonitoringRulesService) DeleteAllAccessMonitoringRules(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

