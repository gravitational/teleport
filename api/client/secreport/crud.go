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

package secreport

import (
	"context"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types/secreports"
	v1 "github.com/gravitational/teleport/api/types/secreports/convert/v1"
)

// GetSecurityAuditQuery returns audit query by name
func (c *Client) GetSecurityAuditQuery(ctx context.Context, name string) (*secreports.AuditQuery, error) {
	resp, err := c.grpcClient.GetAuditQuery(ctx, &pb.GetAuditQueryRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	out, err := v1.FromProtoAuditQuery(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// UpsertSecurityAuditQuery upsets audit query.
func (c *Client) UpsertSecurityAuditQuery(ctx context.Context, in *secreports.AuditQuery) error {
	_, err := c.grpcClient.UpsertAuditQuery(ctx, &pb.UpsertAuditQueryRequest{AuditQuery: v1.ToProtoAuditQuery(in)})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteSecurityAuditQuery deletes audit query by name.
func (c *Client) DeleteSecurityAuditQuery(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteAuditQuery(ctx, &pb.DeleteAuditQueryRequest{Name: name})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// UpsertSecurityReport upsets security report.
func (c *Client) UpsertSecurityReport(ctx context.Context, item *secreports.Report) error {
	_, err := c.grpcClient.UpsertReport(ctx, &pb.UpsertReportRequest{Report: v1.ToProtoReport(item)})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// GetSecurityReport returns security report by name.
func (c *Client) GetSecurityReport(ctx context.Context, name string) (*secreports.Report, error) {
	resp, err := c.grpcClient.GetReport(ctx, &pb.GetReportRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	out, err := v1.FromProtoReport(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// GetSecurityReportResult returns security report details by name.
func (c *Client) GetSecurityReportResult(ctx context.Context, name string, days int) (*pb.ReportResult, error) {
	resp, err := c.grpcClient.GetReportResult(ctx, &pb.GetReportResultRequest{
		Name: name,
		Days: uint32(days),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.GetResult(), nil
}

// RunSecurityReport runs security report by name.
func (c *Client) RunSecurityReport(ctx context.Context, name string, days int) error {
	_, err := c.grpcClient.RunReport(ctx, &pb.RunReportRequest{Name: name, Days: uint32(days)})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// GetSecurityAuditQueryResult returns audit query result by id.
func (c *Client) GetSecurityAuditQueryResult(ctx context.Context, resultID, nextToken string, maxResults int32) (*pb.GetAuditQueryResultResponse, error) {
	resp, err := c.grpcClient.GetAuditQueryResult(ctx, &pb.GetAuditQueryResultRequest{
		ResultId:   resultID,
		NextToken:  nextToken,
		MaxResults: maxResults,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// UpsertSecurityReportsState upserts security reports state.
func (c *Client) UpsertSecurityReportsState(ctx context.Context, item *secreports.ReportState) error {
	return trace.NotImplemented("UpsertSecurityReportsState is not supported in the gRPC client")
}

func (c *Client) GetSecurityReportState(ctx context.Context, name string) (*secreports.ReportState, error) {
	resp, err := c.grpcClient.GetReportState(ctx, &pb.GetReportStateRequest{Name: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := v1.FromProtoReportState(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}
