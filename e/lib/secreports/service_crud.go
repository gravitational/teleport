package secreports

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/secreports/convert/v1"
	"github.com/gravitational/teleport/e/lib/secreports/reports"
	"github.com/gravitational/teleport/gen/go/eventschema"
)

// UpsertAuditQuery updates the audit query.
func (s *Service) UpsertAuditQuery(ctx context.Context, req *pb.UpsertAuditQueryRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbUpdate, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.upsertAuditQuery(ctx, req); err != nil {
		s.log.WarnContext(ctx, "Failed to upsert audit query", "error", err)
		switch {
		case trace.IsBadParameter(err):
			return nil, trace.Wrap(err)
		default:
			return nil, errors.New("failed to upsert audit query")
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) upsertAuditQuery(ctx context.Context, req *pb.UpsertAuditQueryRequest) error {
	item, err := conv.FromProtoAuditQuery(req.GetAuditQuery())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.storage.UpsertSecurityAuditQuery(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAuditQuery returns the audit query.
func (s *Service) GetAuditQuery(ctx context.Context, req *pb.GetAuditQueryRequest) (*pb.AuditQuery, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.storage.GetSecurityAuditQuery(ctx, req.GetName())
	if err != nil {
		s.log.DebugContext(ctx, "Failed to get audit query", "error", err)
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("audit query %s not found", req.GetName())
		default:
			return nil, errors.New("failed to get audit query")
		}
	}
	return conv.ToProtoAuditQuery(item), nil
}

// ListAuditQueries list audit queries.
func (s *Service) ListAuditQueries(ctx context.Context, req *pb.ListAuditQueriesRequest) (*pb.ListAuditQueriesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	items, nextToken, err := s.storage.ListSecurityAuditQueries(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp := &pb.ListAuditQueriesResponse{
		Queries:       toProtoAuditQueries(items),
		NextPageToken: nextToken,
	}
	return resp, nil
}

// GetSchema returns the audit query schema.
func (s *Service) GetSchema(ctx context.Context, _ *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	eventSchema, err := eventschema.GetViewsDetails()
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get schema", "error", err)
		return nil, trace.Wrap(err)
	}
	return &pb.GetSchemaResponse{
		Views: toProtoTableSchemaDetails(eventSchema),
	}, nil
}

// DeleteAuditQuery deletes the Audit Query.
func (s *Service) DeleteAuditQuery(ctx context.Context, req *pb.DeleteAuditQueryRequest) (*emptypb.Empty, error) {
	if err := validateRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAuditQuery, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteSecurityAuditQuery(ctx, req.GetName()); err != nil {
		s.log.WarnContext(ctx, "Failed to delete audit query", "error", err)
		switch {
		case trace.IsBadParameter(err):
			return nil, trace.Wrap(err)
		default:
			return nil, errors.New("failed to delete audit query")
		}
	}
	return &emptypb.Empty{}, nil
}

// UpsertReport creates or updates Security Report.
func (s *Service) UpsertReport(ctx context.Context, req *pb.UpsertReportRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbUpdate, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if reports.IsPreBuiltReport(req.GetReport().Header.GetMetadata().GetName()) {
		return nil, trace.BadParameter("cannot modify pre-build report")
	}

	if err := s.upsertSecurityReport(ctx, req); err != nil {
		s.log.WarnContext(ctx, "Failed to upsert security report", "error", err)
		switch {
		case trace.IsBadParameter(err):
			return nil, trace.Wrap(err)
		default:
			return nil, errors.New("failed to upsert security report")
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) upsertSecurityReport(ctx context.Context, req *pb.UpsertReportRequest) error {
	item, err := conv.FromProtoReport(req.GetReport())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.storage.UpsertSecurityReport(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetReport returns the security Report.
func (s *Service) GetReport(ctx context.Context, req *pb.GetReportRequest) (*pb.Report, error) {
	if err := validateRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.storage.GetSecurityReport(ctx, req.GetName())
	if err != nil {
		s.log.WarnContext(ctx, "Failed to get security report", "error", err)
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("security report %s not found", req.GetName())
		default:
			return nil, errors.New("failed to get report")
		}
	}
	return conv.ToProtoReport(item), nil
}

// ListReports list the security reports.
func (s *Service) ListReports(ctx context.Context, req *pb.ListReportsRequest) (*pb.ListReportsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := s.listSecurityReports(ctx, req)
	if err != nil {
		s.log.WarnContext(ctx, "Failed to list ListReports", "error", err)
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (s *Service) listSecurityReports(ctx context.Context, req *pb.ListReportsRequest) (*pb.ListReportsResponse, error) {
	items, nextToken, err := s.storage.ListSecurityReports(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var reps []*pb.Report
	for _, v := range items {
		reps = append(reps, conv.ToProtoReport(v))
	}
	resp := &pb.ListReportsResponse{
		Reports:       reps,
		NextPageToken: nextToken,
	}
	return resp, nil
}

// DeleteReport deletes the Security Report.
func (s *Service) DeleteReport(ctx context.Context, req *pb.DeleteReportRequest) (*emptypb.Empty, error) {
	if err := validateRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindSecurityReport, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.deleteSecurityReport(ctx, req); err != nil {
		s.log.WarnContext(ctx, "Failed to delete security report", "error", err)
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("report %s not found", req.GetName())
		case trace.IsBadParameter(err):
			return nil, trace.Wrap(err)
		default:
			return nil, errors.New("failed to delete security report")
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) deleteSecurityReport(ctx context.Context, req *pb.DeleteReportRequest) error {
	if reports.IsPreBuiltReport(req.GetName()) {
		return trace.BadParameter("cannot delete pre-build report")
	}
	if err := s.storage.DeleteSecurityReport(ctx, req.GetName()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
