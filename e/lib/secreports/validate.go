package secreports

import (
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
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
	case *pb.RunReportRequest:
		if t.Name == "" {
			return trace.BadParameter("missing name")
		}
		if ok := slices.Contains(reportValidDaysRange, int32(t.Days)); !ok {
			return trace.BadParameter("days must be one of %v", reportValidDaysRange)
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
