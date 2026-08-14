package secreports

import (
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types/secreports"
	conv "github.com/gravitational/teleport/api/types/secreports/convert/v1"
	"github.com/gravitational/teleport/gen/go/eventschema"
)

func toProtoTableSchemaDetails(in []*eventschema.TableSchemaDetails) []*pb.GetSchemaResponse_ViewDesc {
	out := make([]*pb.GetSchemaResponse_ViewDesc, 0, len(in))
	for _, v := range in {
		view := pb.GetSchemaResponse_ViewDesc_builder{
			Name: v.SQLViewName,
			Desc: v.Description,
		}.Build()
		for _, c := range v.Columns {
			view.SetColumns(append(view.GetColumns(), pb.GetSchemaResponse_ViewDesc_ColumnDesc_builder{
				Name: c.NameSQL(),
				Type: c.Type,
				Desc: c.Description,
			}.Build()))
		}
		out = append(out, view)
	}
	return out
}

func toProtoAuditQueries(in []*secreports.AuditQuery) []*pb.AuditQuery {
	out := make([]*pb.AuditQuery, 0, len(in))
	for _, v := range in {
		out = append(out, conv.ToProtoAuditQuery(v))
	}
	return out
}
