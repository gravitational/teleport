// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		view := &pb.GetSchemaResponse_ViewDesc{
			Name: v.SQLViewName,
			Desc: v.Description,
		}
		for _, c := range v.Columns {
			view.Columns = append(view.Columns, &pb.GetSchemaResponse_ViewDesc_ColumnDesc{
				Name: c.NameSQL(),
				Type: c.Type,
				Desc: c.Description,
			})
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
