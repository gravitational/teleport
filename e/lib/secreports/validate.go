package secreports

import (
	"slices"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func validateRequest(req any, f modules.Features) error {
	switch t := req.(type) {
	case *pb.RunAuditQueryRequest:
		if t.GetQuery() == "" {
			return trace.BadParameter("missing query")
		}
		if ok := slices.Contains(reportValidDaysRange, t.GetDays()); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(t.GetDays(), f); err != nil {
			return trace.Wrap(err)
		}

	case *pb.GetAuditQueryResultRequest:
		if t.GetResultId() == "" {
			return trace.BadParameter("missing result id")
		}
	case *pb.GetReportStateRequest:
		if t.GetName() == "" {
			return trace.BadParameter("missing name")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.GetDays())); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(int32(t.GetDays()), f); err != nil {
			return trace.Wrap(err)
		}
	case *pb.GetReportResultRequest:
		if t.GetName() == "" {
			return trace.BadParameter("missing name")
		}
		if t.GetDays() == 0 {
			return trace.BadParameter("days must be greater than 0")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.GetDays())); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(int32(t.GetDays()), f); err != nil {
			return trace.Wrap(err)
		}
	case *pb.RunReportRequest:
		if t.GetName() == "" {
			return trace.BadParameter("missing name")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.GetDays())); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(int32(t.GetDays()), f); err != nil {
			return trace.Wrap(err)
		}
	case *pb.DeleteReportRequest:
		if t.GetName() == "" {
			return trace.BadParameter("missing name")
		}
	case *pb.GetReportRequest:
		if t.GetName() == "" {
			return trace.BadParameter("missing name")
		}
	case *pb.DeleteAuditQueryRequest:
		if t.GetName() == "" {
			return trace.BadParameter("missing name")
		}
	default:
		return trace.BadParameter("unknown request type: %T", req)
	}
	return nil
}

func verifyAccessMonitoringMaxReportRangeLimit(days int32, f modules.Features) error {
	if !f.GetEntitlement(entitlements.AccessMonitoring).Enabled {
		return trace.AccessDenied("access monitoring is not enabled")
	}

	if f.GetEntitlement(entitlements.AccessMonitoring).Enabled && f.GetEntitlement(entitlements.AccessMonitoring).Limit == 0 {
		return nil // any range supported, unlimited access
	}

	if days > f.GetEntitlement(entitlements.AccessMonitoring).Limit {
		return trace.AccessDenied("day range is not supported, please contact the cluster administrator")
	}

	return nil
}
