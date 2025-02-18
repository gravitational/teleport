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
	"slices"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const accessMonitoringRulesPrefix = "access_monitoring_rule"

// AccessMonitoringRulesService manages AccessMonitoringRules in the Backend.
type AccessMonitoringRulesService struct {
	svc *generic.ServiceWrapper[*accessmonitoringrulesv1.AccessMonitoringRule]
}

// NewAccessMonitoringRulesService creates a new AccessMonitoringRulesService.
func NewAccessMonitoringRulesService(b backend.Backend) (*AccessMonitoringRulesService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*accessmonitoringrulesv1.AccessMonitoringRule]{
			Backend:       b,
			ResourceKind:  types.KindAccessMonitoringRule,
			BackendPrefix: backend.NewKey(accessMonitoringRulesPrefix),
			MarshalFunc:   services.MarshalAccessMonitoringRule,
			UnmarshalFunc: services.UnmarshalAccessMonitoringRule,
			ValidateFunc:  services.ValidateAccessMonitoringRule,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AccessMonitoringRulesService{
		svc: service,
	}, nil
}

// ListAccessMonitoringRules returns a paginated list of AccessMonitoringRule resources.
func (s *AccessMonitoringRulesService) ListAccessMonitoringRules(ctx context.Context, pageSize int, pageToken string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	igs, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return igs, nextKey, nil
}

// GetAccessMonitoringRule returns the specified AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	ig, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ig, nil
}

// CreateAccessMonitoringRule creates a new AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) CreateAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	created, err := s.svc.CreateResource(ctx, amr)
	return created, trace.Wrap(err)
}

// UpdateAccessMonitoringRule updates an existing AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) UpdateAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	updated, err := s.svc.UnconditionalUpdateResource(ctx, amr)
	return updated, trace.Wrap(err)
}

// UpsertAccessMonitoringRule upserts an existing AccessMonitoringRule resource.
func (s *AccessMonitoringRulesService) UpsertAccessMonitoringRule(ctx context.Context, amr *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
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

// ListAccessMonitoringRulesWithFilter returns a paginated list of access monitoring rules that match the given filter.
func (s *AccessMonitoringRulesService) ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	resources, nextKey, err := s.svc.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(),
		func(resource *accessmonitoringrulesv1.AccessMonitoringRule) bool {
			return match(resource, req.GetSubjects(), req.GetNotificationName(), req.GetAutomaticApprovalName())
		})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resources, nextKey, nil
}

func match(rule *accessmonitoringrulesv1.AccessMonitoringRule, subjects []string, notificationName, automaticApprovalName string) bool {
	if notificationName != "" {
		if rule.Spec.Notification == nil || rule.Spec.Notification.Name != notificationName {
			return false
		}
	}
	if automaticApprovalName != "" {
		if rule.GetSpec().GetAutomaticApproval().GetName() != automaticApprovalName {
			return false
		}
	}
	for _, subject := range subjects {
		if ok := slices.ContainsFunc(rule.Spec.Subjects, func(s string) bool {
			return s == subject
		}); !ok {
			return false
		}
	}
	return true
}
