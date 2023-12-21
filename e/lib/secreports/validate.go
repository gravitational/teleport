package secreports

import (
	"slices"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/lib/modules"
)

func validateRequest(req any) error {
	switch t := req.(type) {
	case *pb.RunAuditQueryRequest:
		if t.Query == "" {
			return trace.BadParameter("missing query")
		}
		if ok := slices.Contains(reportValidDaysRange, t.Days); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(t.Days); err != nil {
			return trace.Wrap(err)
		}

	case *pb.GetAuditQueryResultRequest:
		if t.ResultId == "" {
			return trace.BadParameter("missing result id")
		}
	case *pb.GetReportStateRequest:
		if t.Name == "" {
			return trace.BadParameter("missing name")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.Days)); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(int32(t.Days)); err != nil {
			return trace.Wrap(err)
		}
	case *pb.GetReportResultRequest:
		if t.Name == "" {
			return trace.BadParameter("missing name")
		}
		if t.Days == 0 {
			return trace.BadParameter("days must be greater than 0")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.Days)); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(int32(t.Days)); err != nil {
			return trace.Wrap(err)
		}
	case *pb.RunReportRequest:
		if t.Name == "" {
			return trace.BadParameter("missing name")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.Days)); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
		}
		if err := verifyAccessMonitoringMaxReportRangeLimit(int32(t.Days)); err != nil {
			return trace.Wrap(err)
		}
	case *pb.DeleteReportRequest:
		if t.Name == "" {
			return trace.BadParameter("missing name")
		}
	case *pb.GetReportRequest:
		if t.Name == "" {
			return trace.BadParameter("missing name")
		}

	default:
		return trace.BadParameter("unknown request type: %T", req)
	}
	return nil
}

func verifyAccessMonitoringMaxReportRangeLimit(days int32) error {
	f := modules.GetModules().Features()
	if f.IGSEnabled() {
		return nil // any range supported
	}

	if days > int32(f.AccessMonitoring.MaxReportRangeLimit) {
		return trace.AccessDenied("day range is not supported, please contact the cluster administrator")
	}

	return nil
}
