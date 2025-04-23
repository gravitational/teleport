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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

var (
	// AuditQueryPrefix is the prefix for audit queries.
	AuditQueryPrefix = backend.NewKey("security_report", "audit_query")
	// SecurityReportPrefix is the prefix for security reports.
	SecurityReportPrefix = backend.NewKey("security_report", "report")
	// SecurityReportStatePrefix  is the prefix for security report states.
	SecurityReportStatePrefix = backend.NewKey("security_report", "state")
	// SecurityReportCostLimiterPrefix is the prefix for security report cost limiter.
	SecurityReportCostLimiterPrefix = backend.NewKey("security_report", "cost_limiter")
)

// SecReportsService is the local implementation of the SecReports service.
type SecReportsService struct {
	clock                            clockwork.Clock
	auditQuerySvc                    *generic.Service[*secreports.AuditQuery]
	securityReportSvc                *generic.Service[*secreports.Report]
	securityReportStateSvc           *generic.Service[*secreports.ReportState]
	securityReportCostCostLimiterSvc *generic.Service[*secreports.CostLimiter]
}

// NewSecReportsService returns a new instance of the SecReports service.
func NewSecReportsService(backend backend.Backend, clock clockwork.Clock) (*SecReportsService, error) {
	auditQuerySvc, err := generic.NewService(&generic.ServiceConfig[*secreports.AuditQuery]{
		Backend:       backend,
		ResourceKind:  types.KindAuditQuery,
		BackendPrefix: AuditQueryPrefix,
		MarshalFunc:   services.MarshalAuditQuery,
		UnmarshalFunc: services.UnmarshalAuditQuery,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	securityReportSvc, err := generic.NewService(&generic.ServiceConfig[*secreports.Report]{
		Backend:       backend,
		ResourceKind:  types.KindSecurityReport,
		BackendPrefix: SecurityReportPrefix,
		MarshalFunc:   services.MarshalSecurityReport,
		UnmarshalFunc: services.UnmarshalSecurityReport,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	securityReportStateSvc, err := generic.NewService(&generic.ServiceConfig[*secreports.ReportState]{
		Backend:       backend,
		ResourceKind:  types.KindSecurityReportState,
		BackendPrefix: SecurityReportStatePrefix,
		MarshalFunc:   services.MarshalSecurityReportState,
		UnmarshalFunc: services.UnmarshalSecurityReportState,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	costSvc, err := generic.NewService(&generic.ServiceConfig[*secreports.CostLimiter]{
		Backend:       backend,
		ResourceKind:  types.KindSecurityReportCostLimiter,
		BackendPrefix: SecurityReportCostLimiterPrefix,
		MarshalFunc:   services.MarshalSecurityCostLimiter,
		UnmarshalFunc: services.UnmarshalSecurityCostLimiter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SecReportsService{
		clock:                            clock,
		auditQuerySvc:                    auditQuerySvc,
		securityReportSvc:                securityReportSvc,
		securityReportStateSvc:           securityReportStateSvc,
		securityReportCostCostLimiterSvc: costSvc,
	}, nil
}

// UpsertSecurityAuditQuery upserts audit query.
func (s *SecReportsService) UpsertSecurityAuditQuery(ctx context.Context, in *secreports.AuditQuery) error {
	_, err := s.auditQuerySvc.UpsertResource(ctx, in)
	return trace.Wrap(err)
}

// GetSecurityAuditQueries returns audit queries.
func (s *SecReportsService) GetSecurityAuditQueries(ctx context.Context) ([]*secreports.AuditQuery, error) {
	audits, err := s.auditQuerySvc.GetResources(ctx)
	return audits, trace.Wrap(err)
}

// GetSecurityReports returns security reports.
func (s *SecReportsService) GetSecurityReports(ctx context.Context) ([]*secreports.Report, error) {
	reports, err := s.securityReportSvc.GetResources(ctx)
	return reports, trace.Wrap(err)
}

// GetSecurityReportsStates returns security report states.
func (s *SecReportsService) GetSecurityReportsStates(ctx context.Context) ([]*secreports.ReportState, error) {
	states, err := s.securityReportStateSvc.GetResources(ctx)
	return states, trace.Wrap(err)
}

// GetSecurityAuditQuery returns audit query by name.
func (s *SecReportsService) GetSecurityAuditQuery(ctx context.Context, name string) (*secreports.AuditQuery, error) {
	r, err := s.auditQuerySvc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}

// ListSecurityAuditQueries returns a list of audit queries.
func (s *SecReportsService) ListSecurityAuditQueries(ctx context.Context, pageSize int, nextToken string) ([]*secreports.AuditQuery, string, error) {
	items, nextToken, err := s.auditQuerySvc.ListResources(ctx, pageSize, nextToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextToken, nil
}

// DeleteSecurityAuditQuery deletes audit query by name.
func (s *SecReportsService) DeleteSecurityAuditQuery(ctx context.Context, name string) error {
	return trace.Wrap(s.auditQuerySvc.DeleteResource(ctx, name))
}

// UpsertSecurityReport upserts security report.
func (s *SecReportsService) UpsertSecurityReport(ctx context.Context, item *secreports.Report) error {
	_, err := s.securityReportSvc.UpsertResource(ctx, item)
	return trace.Wrap(err)
}

// GetSecurityReport returns security report by name.
func (s *SecReportsService) GetSecurityReport(ctx context.Context, name string) (*secreports.Report, error) {
	r, err := s.securityReportSvc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}

// ListSecurityReports returns a list of security reports.
func (s *SecReportsService) ListSecurityReports(ctx context.Context, i int, token string) ([]*secreports.Report, string, error) {
	items, nextToken, err := s.securityReportSvc.ListResources(ctx, i, token)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextToken, nil
}

// UpsertSecurityReportsState upserts security report state.
func (s *SecReportsService) UpsertSecurityReportsState(ctx context.Context, item *secreports.ReportState) error {
	_, err := s.securityReportStateSvc.UpsertResource(ctx, item)
	return trace.Wrap(err)
}

// GetSecurityReportState returns security report state by name.
func (s *SecReportsService) GetSecurityReportState(ctx context.Context, name string) (*secreports.ReportState, error) {
	r, err := s.securityReportStateSvc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}

// DeleteSecurityReport deletes security report by name.
func (s *SecReportsService) DeleteSecurityReport(ctx context.Context, name string) error {
	return trace.Wrap(s.securityReportSvc.DeleteResource(ctx, name))
}

// DeleteAllSecurityAuditQueries deletes all audit queries.
func (s *SecReportsService) DeleteAllSecurityAuditQueries(ctx context.Context) error {
	return trace.Wrap(s.auditQuerySvc.DeleteAllResources(ctx))
}

// DeleteAllSecurityReports deletes all security reports.
func (s *SecReportsService) DeleteAllSecurityReports(ctx context.Context) error {
	return trace.Wrap(s.securityReportSvc.DeleteAllResources(ctx))
}

func (s *SecReportsService) ListSecurityReportsStates(ctx context.Context, pageSize int, nextToken string) ([]*secreports.ReportState, string, error) {
	items, nextToken, err := s.securityReportStateSvc.ListResources(ctx, pageSize, nextToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextToken, nil
}

// DeleteSecurityReportsState deletes security report state by name.
func (s *SecReportsService) DeleteSecurityReportsState(ctx context.Context, name string) error {
	return trace.Wrap(s.securityReportStateSvc.DeleteResource(ctx, name))
}

// DeleteAllSecurityReportsStates deletes all security report states.
func (s *SecReportsService) DeleteAllSecurityReportsStates(ctx context.Context) error {
	return trace.Wrap(s.securityReportStateSvc.DeleteAllResources(ctx))
}

// UpsertCostLimiter upserts cost limiter.
func (s *SecReportsService) UpsertCostLimiter(ctx context.Context, item *secreports.CostLimiter) error {
	_, err := s.securityReportCostCostLimiterSvc.UpsertResource(ctx, item)
	return trace.Wrap(err)
}

// GetCostLimiter returns cost limiter by name.
func (s *SecReportsService) GetCostLimiter(ctx context.Context, name string) (*secreports.CostLimiter, error) {
	r, err := s.securityReportCostCostLimiterSvc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}
