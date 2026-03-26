// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package secreportsv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	secreportsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
)

// NotImplementedService is a [secreportsv1pb.SecReportsServiceServer] which
// returns errors for all RPCs which indicate that an enterprise license
// is required to use the service.
type NotImplementedService struct {
	secreportsv1pb.UnsafeSecReportsServiceServer

	// CustomError sets a custom error that the service will return for all
	// methods, it should contain information about why the security reports
	// service is disabled.
	CustomError error
}

func (n NotImplementedService) error() error {
	if n.CustomError == nil {
		return trace.AccessDenied("Security Reports are only available with an enterprise license including Access Monitoring in clusters using the Athena audit backend")
	}
	return trace.Wrap(n.CustomError)
}

// DeleteAuditQuery implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) DeleteAuditQuery(context.Context, *secreportsv1pb.DeleteAuditQueryRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, n.error()
}

// DeleteReport implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) DeleteReport(context.Context, *secreportsv1pb.DeleteReportRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, n.error()
}

// GetAuditQuery implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) GetAuditQuery(context.Context, *secreportsv1pb.GetAuditQueryRequest) (*secreportsv1pb.AuditQuery, error) {
	return nil, n.error()
}

// GetAuditQueryResult implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) GetAuditQueryResult(context.Context, *secreportsv1pb.GetAuditQueryResultRequest) (*secreportsv1pb.GetAuditQueryResultResponse, error) {
	return nil, n.error()
}

// GetReport implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) GetReport(context.Context, *secreportsv1pb.GetReportRequest) (*secreportsv1pb.Report, error) {
	return nil, n.error()
}

// GetReportResult implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) GetReportResult(context.Context, *secreportsv1pb.GetReportResultRequest) (*secreportsv1pb.GetReportResultResponse, error) {
	return nil, n.error()
}

// GetReportState implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) GetReportState(context.Context, *secreportsv1pb.GetReportStateRequest) (*secreportsv1pb.ReportState, error) {
	return nil, n.error()
}

// GetSchema implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) GetSchema(context.Context, *secreportsv1pb.GetSchemaRequest) (*secreportsv1pb.GetSchemaResponse, error) {
	return nil, n.error()
}

// ListAuditQueries implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) ListAuditQueries(context.Context, *secreportsv1pb.ListAuditQueriesRequest) (*secreportsv1pb.ListAuditQueriesResponse, error) {
	return nil, n.error()
}

// ListReportStates implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) ListReportStates(context.Context, *secreportsv1pb.ListReportStatesRequest) (*secreportsv1pb.ListReportStatesResponse, error) {
	return nil, n.error()
}

// ListReports implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) ListReports(context.Context, *secreportsv1pb.ListReportsRequest) (*secreportsv1pb.ListReportsResponse, error) {
	return nil, n.error()
}

// RunAuditQuery implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) RunAuditQuery(context.Context, *secreportsv1pb.RunAuditQueryRequest) (*secreportsv1pb.RunAuditQueryResponse, error) {
	return nil, n.error()
}

// RunReport implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) RunReport(context.Context, *secreportsv1pb.RunReportRequest) (*emptypb.Empty, error) {
	return nil, n.error()
}

// UpsertAuditQuery implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) UpsertAuditQuery(context.Context, *secreportsv1pb.UpsertAuditQueryRequest) (*emptypb.Empty, error) {
	return nil, n.error()
}

// UpsertReport implements secreportsv1.SecReportsServiceServer.
func (n NotImplementedService) UpsertReport(context.Context, *secreportsv1pb.UpsertReportRequest) (*emptypb.Empty, error) {
	return nil, n.error()
}
