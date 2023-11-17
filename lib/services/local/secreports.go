/*
Copyright 2023 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	// AuditQueryPrefix is the prefix for audit queries.
	AuditQueryPrefix = "security_report/audit_query"
	// SecurityReportPrefix is the prefix for security reports.
	SecurityReportPrefix = "security_report/report"
	// SecurityReportStatePrefix  is the prefix for security report states.
	SecurityReportStatePrefix = "security_report/state"
	// SecurityReportCostLimiterPrefix is the prefix for security report cost limiter.
	SecurityReportCostLimiterPrefix = "security_report/cost_limiter"
)

// SecReportsService is the local implementation of the SecReports service.
type SecReportsService struct {
	log                              logrus.FieldLogger
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
		log:                              logrus.WithFields(logrus.Fields{trace.Component: "secreports:local-service"}),
		clock:                            clock,
		auditQuerySvc:                    auditQuerySvc,
		securityReportSvc:                securityReportSvc,
		securityReportStateSvc:           securityReportStateSvc,
		securityReportCostCostLimiterSvc: costSvc,
	}, nil
}

// UpsertSecurityAuditQuery upserts audit query.
func (s *SecReportsService) UpsertSecurityAuditQuery(ctx context.Context, in *secreports.AuditQuery) error {
	if err := s.auditQuerySvc.UpsertResource(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSecurityAuditQueries returns audit queries.
func (s *SecReportsService) GetSecurityAuditQueries(ctx context.Context) ([]*secreports.AuditQuery, error) {
	return s.auditQuerySvc.GetResources(ctx)
}

// GetSecurityReports returns security reports.
func (s *SecReportsService) GetSecurityReports(ctx context.Context) ([]*secreports.Report, error) {
	return s.securityReportSvc.GetResources(ctx)
}

// GetSecurityReportsStates returns security report states.
func (s *SecReportsService) GetSecurityReportsStates(ctx context.Context) ([]*secreports.ReportState, error) {
	return s.securityReportStateSvc.GetResources(ctx)
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
	if err := s.securityReportSvc.UpsertResource(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	if err := s.securityReportStateSvc.UpsertResource(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	if err := s.securityReportCostCostLimiterSvc.UpsertResource(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetCostLimiter returns cost limiter by name.
func (s *SecReportsService) GetCostLimiter(ctx context.Context, name string) (*secreports.CostLimiter, error) {
	r, err := s.securityReportCostCostLimiterSvc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}
