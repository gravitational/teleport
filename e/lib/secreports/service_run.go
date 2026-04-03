package secreports

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/secreports"
	conv "github.com/gravitational/teleport/api/types/secreports/convert/v1"
	"github.com/gravitational/teleport/e/lib/secreports/query"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

// RunAuditQuery runs the audit query.
func (s *Service) RunAuditQuery(ctx context.Context, req *pb.RunAuditQueryRequest) (*pb.RunAuditQueryResponse, error) {
	if err := validateRequest(req, s.modules.Features()); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.userQueriesLimiter.Allow() {
		return nil, trace.LimitExceeded("Limit of users concurrent Access Monitoring queries exceeded. Please try again later.")
	}
	defer s.userQueriesLimiter.Release()

	resp, err := s.runAuditQuery(ctx, req)
	if err != nil {
		s.log.WarnContext(ctx, "Failed to run audit query", "error", err)
		switch {
		case trace.IsBadParameter(err):
			return nil, trace.Wrap(err)
		case trace.IsNotFound(err):
			return nil, trace.Wrap(err)
		default:
			return nil, errors.New("failed to run audit query")
		}
	}
	return resp, nil
}

func (s *Service) runAuditQuery(ctx context.Context, req *pb.RunAuditQueryRequest) (*pb.RunAuditQueryResponse, error) {
	event := &apievents.AuditQueryRun{
		Metadata: apievents.Metadata{
			Type: events.SecReportsAuditQueryRunEvent,
			Code: events.SecReportsAuditQueryRunCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		AuditQueryDetails: apievents.AuditQueryDetails{
			Query: req.Query,
			Days:  req.GetDays(),
		},
	}
	defer func() {
		if err := s.emitter.EmitAuditEvent(ctx, event); err != nil {
			s.log.WarnContext(ctx, "Failed to emit audit event", "error", err)
		}
	}()
	result, err := s.athena.RunQuery(ctx, req.GetQuery(), int(req.Days))
	if err != nil {
		event.Status = apievents.Status{
			Success: false,
		}
		return nil, trace.Wrap(err)
	}
	event.Status = apievents.Status{
		Success: true,
	}
	event.DataScannedInBytes = result.DataScannedInBytes
	event.ExecutionTimeInMillis = result.TotalExecutionTimeInMillis
	return &pb.RunAuditQueryResponse{
		ResultId: result.ResultID,
	}, nil
}

// GetAuditQueryResult returns the audit query result.
func (s *Service) GetAuditQueryResult(ctx context.Context, req *pb.GetAuditQueryResultRequest) (*pb.GetAuditQueryResultResponse, error) {
	if err := validateRequest(req, s.modules.Features()); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := s.athena.GetQueryResult(ctx, req.ResultId, req.NextToken, req.GetMaxResults())
	if err != nil {
		s.log.WarnContext(ctx, "Failed to get audit query result", "error", err)
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("audit query result %q not found", req.ResultId)
		default:
			return nil, errors.New("failed to get audit query result")
		}
	}
	return &pb.GetAuditQueryResultResponse{
		Result:    result.ToProto(),
		NextToken: result.NextToken,
		ResultId:  req.ResultId,
	}, nil
}

// GetReportResult returns security reports result.
func (s *Service) GetReportResult(ctx context.Context, req *pb.GetReportResultRequest) (*pb.GetReportResultResponse, error) {
	if err := validateRequest(req, s.modules.Features()); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	executionName := secreports.ReportExecutionName(req.GetName(), int32(req.GetDays()))
	result, err := s.reportStore.LoadReportResult(ctx, executionName)
	if err != nil {
		s.log.WarnContext(ctx, "Failed to get report result", "error", err)
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("report details %q not found", req.GetName())
		default:
			return nil, trace.Wrap(err)
		}
	}
	return &pb.GetReportResultResponse{
		Result: result,
	}, nil
}

// GetReportState returns security report state.
func (s *Service) GetReportState(ctx context.Context, req *pb.GetReportStateRequest) (*pb.ReportState, error) {
	if err := validateRequest(req, s.modules.Features()); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	executionName := secreports.ReportExecutionName(req.GetName(), int32(req.GetDays()))
	state, err := s.storage.GetSecurityReportState(ctx, executionName)
	if err != nil {
		s.log.WarnContext(ctx, "Failed to get report state", "error", err)
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("report state %q not found", executionName)
		default:
			return nil, trace.Wrap(err)
		}
	}
	return conv.ToProtoReportState(state), nil
}

// RunReport runs the security report.
func (s *Service) RunReport(ctx context.Context, req *pb.RunReportRequest) (*emptypb.Empty, error) {
	if err := validateRequest(req, s.modules.Features()); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	report, err := s.storage.GetSecurityReport(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		// Run report Asynchronously.
		// The result is saved as report state and can be retrieved later.
		if err := s.runReport(s.ParentCtx, report, int32(req.GetDays())); err != nil {
			s.log.WarnContext(ctx, "Failed to run security report", "error", err)
		}
	}()
	return &emptypb.Empty{}, nil
}

func (s *Service) runReportAndUpdateState(ctx context.Context, report *secreports.Report, days int32) (*pb.ReportResult, error) {
	executionName := secreports.ReportExecutionName(report.GetName(), days)
	now := s.clock.Now()
	status := secreports.Failed
	defer func() {
		if err := s.updateReportState(ctx, executionName, status); err != nil {
			s.log.ErrorContext(ctx, "Failed to update report state", "error", err)
		}
	}()

	event := &apievents.SecurityReportRun{
		Metadata: apievents.Metadata{
			Type: events.SecReportsReportRunEvent,
			Code: events.SecReportsReportRunCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		Status: apievents.Status{
			Success: true,
		},
		Name: executionName,
	}
	defer func() {
		if err := s.emitter.EmitAuditEvent(ctx, event); err != nil {
			s.log.ErrorContext(ctx, "Failed to emit audit event", "error", err)
		}
	}()

	runResult, err := s.runReportAndCollectResult(ctx, report, days)
	if err != nil {
		event.Status.Success = false
		return nil, trace.Wrap(err)
	}
	s.log.DebugContext(ctx,
		"Report was successfully executed",
		"report_name", executionName,
		"total_data_scanned", runResult.TotalDataScannedInBytes,
		"total_execution_time", runResult.TotalExecutionTimeInMillis,
		"duration", s.clock.Since(now),
	)
	event.TotalExecutionTimeInMillis = runResult.TotalExecutionTimeInMillis
	event.TotalDataScannedInBytes = runResult.TotalDataScannedInBytes
	if err = s.reportStore.SaveReportResult(ctx, executionName, runResult); err != nil {
		return nil, trace.Wrap(err)
	}
	status = secreports.Ready
	return runResult, nil
}

// shouldRunReport returns true if report should be executed.
// Depending on the report state and recent updatedAt time report should be executed.
func (s *Service) shouldRunReport(ctx context.Context, executionName string, triggerThreshold time.Duration) (bool, error) {
	state, err := s.storage.GetSecurityReportState(ctx, executionName)
	if err != nil {
		if trace.IsNotFound(err) {
			// Reports was not found which means it was never executed
			// or backend item expired. In that case report should be executed.
			s.log.DebugContext(ctx, "Report state was not found", "report", executionName)
			return true, nil
		}
		return false, trace.Wrap(err)
	}
	updatedAt := state.Spec.UpdatedAt

	now := s.clock.Now()
	switch state.Spec.Status {
	case secreports.Failed, secreports.Ready:
		return now.After(updatedAt.Add(triggerThreshold)), nil
	case secreports.Running:
		if now.After(updatedAt.Add(time.Hour)) {
			// If for some reason auth service was not graceful shutdown during Running report phase
			// the Running status can be stale. In that case report should be re-executed.
			s.log.WarnContext(ctx, "Re-executing report that has been running for more than an hour", "report", executionName)
			return true, nil
		}
		s.log.DebugContext(ctx, "Skipping execution of report in running state", "report", executionName)
		return false, nil
	default:
		return true, nil
	}
}

func (s *Service) runReportAndCollectResult(ctx context.Context, report *secreports.Report, days int32) (*pb.ReportResult, error) {
	var (
		totalDataScannedInBytes atomic.Int64
		totalExecutionTime      atomic.Int64
		g                       errgroup.Group
	)
	maxQueryExecutionGoroutines := 4
	g.SetLimit(maxQueryExecutionGoroutines)
	auditQueriesResult := make([]*pb.ReportResult_AuditQueryResult, len(report.Spec.AuditQueries))
	for i, spec := range report.Spec.AuditQueries {
		i := i
		g.Go(func() error {
			result, state, err := s.execAuditQuery(ctx, spec, days)
			if err != nil {
				return trace.Wrap(err)
			}
			auditQueriesResult[i] = &pb.ReportResult_AuditQueryResult{
				AuditQuery: &pb.AuditQuerySpec{
					Name:        spec.Name,
					Title:       spec.Title,
					Query:       spec.Query,
					Description: spec.Description,
				},
				Result:                result,
				ResultId:              state.ResultID,
				ExecutionTimeInMillis: state.TotalExecutionTimeInMillis,
				DataScannedInBytes:    state.DataScannedInBytes,
			}
			totalExecutionTime.Add(state.TotalExecutionTimeInMillis)
			totalDataScannedInBytes.Add(state.DataScannedInBytes)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.ReportResult{
		Name:                       report.GetName(),
		Description:                report.Spec.Description,
		AuditQueryResults:          auditQueriesResult,
		UpdatedAt:                  s.clock.Now().Format(time.RFC3339),
		TotalDataScannedInBytes:    totalDataScannedInBytes.Load(),
		TotalExecutionTimeInMillis: totalExecutionTime.Load(),
	}, nil
}

func (s *Service) execAuditQuery(ctx context.Context, spec *secreports.AuditQuerySpec, days int32) (*pb.QueryResultSet, *query.RunQueryResponse, error) {
	var (
		out       pb.QueryResultSet
		nextToken string
	)
	runResp, err := s.athena.RunQuery(ctx, spec.Query, int(days))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	for {
		resultResp, err := s.athena.GetQueryResult(ctx, runResp.ResultID, nextToken, 0)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		p := resultResp.ToProto()
		if len(out.GetColumnInfo()) == 0 {
			out.ColumnInfo = p.ColumnInfo
		}
		out.Rows = append(out.Rows, p.Rows...)
		if !resultResp.HasMoreData() {
			break
		}
		nextToken = resultResp.NextToken
	}
	return &out, runResp, nil
}
