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

package simple

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accessmonitoringrule"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	accessMonitoringRulePrefix      = "access_monitoring_rule"
	accessMonitoringRuleMaxPageSize = 100
)

// AccessMonitoringRuleService is a simple access monitoring rules backend service for use specifically by the cache.
type AccessMonitoringRuleService struct {
	log     logrus.FieldLogger
	service *generic.Service[*accessmonitoringrule.AccessMonitoringRule]
}

// Create a new simple access monitoring rules backend service for use specifically by the cache.
func NewAccessMonitoringRuleService(backend backend.Backend) (*AccessMonitoringRuleService, error) {
	service, err := generic.NewService(&generic.ServiceConfig[*accessmonitoringrule.AccessMonitoringRule]{
		Backend:       backend,
		PageLimit:     accessMonitoringRuleMaxPageSize,
		ResourceKind:  types.KindAccessMonitoringRule,
		BackendPrefix: accessMonitoringRulePrefix,
		MarshalFunc:   services.MarshalAccessMonitoringRule,
		UnmarshalFunc: services.UnmarshalAccessMonitoringRule,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessMonitoringRuleService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "access-monitoring_rule:simple-service"}),
		service: service,
	}, nil
}

// CreateAccessMonitoringRule creates the specified access monitoring rule.
func (a *AccessMonitoringRuleService) CreateAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error) {
	resource, err := a.service.CreateResource(ctx, amr)
	return resource, trace.Wrap(err)
}

func (a *AccessMonitoringRuleService) GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrule.AccessMonitoringRule, error) {
	resource, err := a.service.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resource, nil
}

// DeleteAccessMonitoringRule deletes the specified access monitoring rule.
func (a *AccessMonitoringRuleService) DeleteAccessMonitoringRule(ctx context.Context, name string) error {
	return trace.Wrap(a.service.DeleteResource(ctx, name))
}

// DeleteAllAccessMonitoringRules deletes all access monitoring rules.
func (a *AccessMonitoringRuleService) DeleteAllAccessMonitoringRules(ctx context.Context) error {
	return trace.Wrap(a.service.DeleteAllResources(ctx))
}

// UpsertAccessMonitoringRule upserts the specified access monitoring rule.
func (a *AccessMonitoringRuleService) UpsertAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error) {
	resource, err := a.service.UpsertResource(ctx, amr)
	return resource, trace.Wrap(err)
}

// ListAccessMonitoringRule lists current access monitoring rules.
func (a *AccessMonitoringRuleService) ListAccessMonitoringRules(ctx context.Context, pageSize int, nextToken string) ([]*accessmonitoringrule.AccessMonitoringRule, string, error) {
	return a.service.ListResources(ctx, pageSize, nextToken)
}
