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

package v1

import (
	"time"

	"github.com/gravitational/trace"

	secreportsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	"github.com/gravitational/teleport/api/types/secreports"
)

// FromProtoAuditQuery converts the audit query from proto.
func FromProtoAuditQuery(in *secreportsv1.AuditQuery) (*secreports.AuditQuery, error) {
	spec := secreports.AuditQuerySpec{
		Name:        in.GetSpec().GetName(),
		Title:       in.GetSpec().GetTitle(),
		Query:       in.GetSpec().GetQuery(),
		Description: in.GetSpec().GetDescription(),
	}
	out, err := secreports.NewAuditQuery(headerv1.FromMetadataProto(in.Header.Metadata), spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// FromProtoAuditQueries converts the audit queries from proto.
func FromProtoAuditQueries(in []*secreportsv1.AuditQuery) ([]*secreports.AuditQuery, error) {
	out := make([]*secreports.AuditQuery, 0, len(in))
	for _, v := range in {
		item, err := FromProtoAuditQuery(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, nil
}

// ToProtoAuditQuery converts the audit query to proto.
func ToProtoAuditQuery(in *secreports.AuditQuery) *secreportsv1.AuditQuery {
	return &secreportsv1.AuditQuery{
		Header: headerv1.ToResourceHeaderProto(in.ResourceHeader),
		Spec: &secreportsv1.AuditQuerySpec{
			Name:        in.Spec.Name,
			Title:       in.Spec.Title,
			Query:       in.Spec.Query,
			Description: in.Spec.Description,
		},
	}
}

// FromProtoReport converts the security report from proto.
func FromProtoReport(in *secreportsv1.Report) (*secreports.Report, error) {
	spec := secreports.ReportSpec{
		Name:         in.GetSpec().GetName(),
		Description:  in.GetSpec().GetDescription(),
		AuditQueries: ToProtoAudiQueriesSpec(in.Spec.GetAuditQueries()),
		Title:        in.GetSpec().GetTitle(),
		Version:      in.GetSpec().GetVersion(),
	}
	out, err := secreports.NewReport(headerv1.FromMetadataProto(in.Header.Metadata), spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// FromProtoReports converts the security reports from proto.
func FromProtoReports(in []*secreportsv1.Report) ([]*secreports.Report, error) {
	out := make([]*secreports.Report, 0, len(in))
	for _, v := range in {
		item, err := FromProtoReport(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, nil
}

// ToProtoAudiQueriesSpec converts the audit queries spec to proto.
func ToProtoAudiQueriesSpec(in []*secreportsv1.AuditQuerySpec) []*secreports.AuditQuerySpec {
	out := make([]*secreports.AuditQuerySpec, 0, len(in))
	for _, v := range in {
		out = append(out, &secreports.AuditQuerySpec{
			Name:        v.GetName(),
			Title:       v.GetTitle(),
			Description: v.GetDescription(),
			Query:       v.GetQuery(),
		})
	}
	return out
}

// ToProtoReport converts the security report to proto.
func ToProtoReport(in *secreports.Report) *secreportsv1.Report {
	return &secreportsv1.Report{
		Header: headerv1.ToResourceHeaderProto(in.ResourceHeader),
		Spec: &secreportsv1.ReportSpec{
			Name:         in.GetName(),
			Description:  in.Spec.Description,
			AuditQueries: FromProtoAuditQueriesSpec(in.Spec.AuditQueries),
			Title:        in.Spec.Title,
			Version:      in.Spec.Version,
		},
	}
}

// FromProtoAuditQueriesSpec converts the audit queries spec from proto.
func FromProtoAuditQueriesSpec(in []*secreports.AuditQuerySpec) []*secreportsv1.AuditQuerySpec {
	out := make([]*secreportsv1.AuditQuerySpec, 0, len(in))
	for _, v := range in {
		out = append(out, &secreportsv1.AuditQuerySpec{
			Name:        v.Name,
			Title:       v.Title,
			Query:       v.Query,
			Description: v.Description,
		})
	}
	return out
}

// FromProtoReportStates converts the security report states from proto.
func FromProtoReportStates(in []*secreportsv1.ReportState) ([]*secreports.ReportState, error) {
	out := make([]*secreports.ReportState, 0, len(in))
	for _, v := range in {
		item, err := FromProtoReportState(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, nil
}

// FromProtoReportState converts the security report state from proto.
func FromProtoReportState(in *secreportsv1.ReportState) (*secreports.ReportState, error) {
	t, err := time.Parse(time.RFC3339, in.GetSpec().GetUpdatedAt())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec := secreports.ReportStateSpec{
		Status:    secreports.Status(in.GetSpec().GetState()),
		UpdatedAt: t.UTC(),
	}
	out, err := secreports.NewReportState(headerv1.FromMetadataProto(in.GetHeader().GetMetadata()), spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// ToProtoReportState converts the security report state to proto.
func ToProtoReportState(in *secreports.ReportState) *secreportsv1.ReportState {
	return &secreportsv1.ReportState{
		Header: headerv1.ToResourceHeaderProto(in.ResourceHeader),
		Spec: &secreportsv1.ReportStateSpec{
			State:     string(in.Spec.Status),
			UpdatedAt: in.Spec.UpdatedAt.Format(time.RFC3339),
		},
	}
}
