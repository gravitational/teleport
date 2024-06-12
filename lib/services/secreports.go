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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/utils"
)

// SecurityAuditQueryGetter is the interface for audit query getters.
type SecurityAuditQueryGetter interface {
	// GetSecurityAuditQuery returns an audit query.
	GetSecurityAuditQuery(ctx context.Context, name string) (*secreports.AuditQuery, error)
	// GetSecurityAuditQueries returns all audit queries.
	GetSecurityAuditQueries(context.Context) ([]*secreports.AuditQuery, error)
	// ListSecurityAuditQueries lists audit queries.
	ListSecurityAuditQueries(context.Context, int, string) ([]*secreports.AuditQuery, string, error)
}

// SecurityReportGetter is the interface for security report getters.
type SecurityReportGetter interface {
	// GetSecurityReport returns a security report.
	GetSecurityReport(ctx context.Context, name string) (*secreports.Report, error)
	// GetSecurityReports returns a security report.
	GetSecurityReports(ctx context.Context) ([]*secreports.Report, error)
	// ListSecurityReports lists security reports.
	ListSecurityReports(ctx context.Context, i int, token string) ([]*secreports.Report, string, error)
}

// SecurityReportStateGetter is the interface for security report state getters.
type SecurityReportStateGetter interface {
	// GetSecurityReportState returns a security report state.
	GetSecurityReportState(ctx context.Context, name string) (*secreports.ReportState, error)
	// GetSecurityReportsStates returns security report states.
	GetSecurityReportsStates(context.Context) ([]*secreports.ReportState, error)
	// ListSecurityReportsStates  lists security report states.
	ListSecurityReportsStates(context.Context, int, string) ([]*secreports.ReportState, string, error)
}

// SecReports is the interface for the SecReports service.
type SecReports interface {
	SecurityAuditQueryGetter
	// UpsertSecurityAuditQuery upserts an audit query.
	UpsertSecurityAuditQuery(ctx context.Context, in *secreports.AuditQuery) error
	// DeleteSecurityAuditQuery deletes an audit query.
	DeleteSecurityAuditQuery(ctx context.Context, name string) error
	// DeleteAllSecurityAuditQueries deletes all audit queries.
	DeleteAllSecurityAuditQueries(context.Context) error

	SecurityReportGetter
	// UpsertSecurityReport upserts a security report.
	UpsertSecurityReport(ctx context.Context, item *secreports.Report) error
	// DeleteSecurityReport deletes a security report.
	DeleteSecurityReport(ctx context.Context, name string) error
	// DeleteAllSecurityReports deletes all audit queries.
	DeleteAllSecurityReports(context.Context) error

	SecurityReportStateGetter
	// UpsertSecurityReportsState upserts a security report state.
	UpsertSecurityReportsState(ctx context.Context, item *secreports.ReportState) error
	// DeleteSecurityReportsState deletes all audit queries.
	DeleteSecurityReportsState(ctx context.Context, name string) error
	// DeleteAllSecurityReportsStates deletes all audit queries.
	DeleteAllSecurityReportsStates(context.Context) error
}

// CostLimiter is the interface for the security cost limiter.
type CostLimiter interface {
	// UpsertCostLimiter upserts a security cost limiter.
	UpsertCostLimiter(ctx context.Context, item *secreports.CostLimiter) error
	// GetCostLimiter returns a security cost limiter.
	GetCostLimiter(ctx context.Context, name string) (*secreports.CostLimiter, error)
}

// MarshalAuditQuery marshals an audit query.
func MarshalAuditQuery(in *secreports.AuditQuery, opts ...MarshalOption) ([]byte, error) {
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *in
		copy.SetResourceID(0)
		in = &copy
	}
	return utils.FastMarshal(in)
}

// UnmarshalAuditQuery unmarshals an audit query.
func UnmarshalAuditQuery(data []byte, opts ...MarshalOption) (*secreports.AuditQuery, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *secreports.AuditQuery
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}

// MarshalSecurityReport marshals a security report.
func MarshalSecurityReport(in *secreports.Report, opts ...MarshalOption) ([]byte, error) {
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *in
		copy.SetResourceID(0)
		in = &copy
	}
	return utils.FastMarshal(in)
}

// UnmarshalSecurityReport unmarshals a security report.
func UnmarshalSecurityReport(data []byte, opts ...MarshalOption) (*secreports.Report, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *secreports.Report
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}

// MarshalSecurityReportState marshals a security report state.
func MarshalSecurityReportState(in *secreports.ReportState, opts ...MarshalOption) ([]byte, error) {
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		copy := *in
		copy.SetResourceID(0)
		in = &copy
	}
	return utils.FastMarshal(in)
}

// UnmarshalSecurityReportState unmarshals a security report state.
func UnmarshalSecurityReportState(data []byte, opts ...MarshalOption) (*secreports.ReportState, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *secreports.ReportState
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}

// MarshalSecurityCostLimiter marshals a security report state.
func MarshalSecurityCostLimiter(in *secreports.CostLimiter, opts ...MarshalOption) ([]byte, error) {
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		copy := *in
		copy.SetResourceID(0)
		in = &copy
	}
	return utils.FastMarshal(in)
}

// UnmarshalSecurityCostLimiter unmarshals a security report cost limiter.
func UnmarshalSecurityCostLimiter(data []byte, opts ...MarshalOption) (*secreports.CostLimiter, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *secreports.CostLimiter
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}
