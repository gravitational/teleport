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
	"github.com/gravitational/trace/trail"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types/secreports"
	v1 "github.com/gravitational/teleport/api/types/secreports/convert/v1"
)

// Client is a gRPC implementation of SecReportsService.
type Client struct {
	grpcClient pb.SecReportsServiceClient
}

func (c *Client) GetSecurityAuditQueries(ctx context.Context) ([]*secreports.AuditQuery, error) {
	var items []*pb.AuditQuery
	nextKey := ""
	for {
		resp, err := c.grpcClient.ListAuditQueries(ctx, &pb.ListAuditQueriesRequest{
			PageSize:  0,
			PageToken: nextKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items = append(items, resp.GetQueries()...)
		if resp.GetNextPageToken() == "" {
			break
		}
		nextKey = resp.GetNextPageToken()
	}
	out, err := v1.FromProtoAuditQueries(items)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (c *Client) ListSecurityAuditQueries(ctx context.Context, size int, token string) ([]*secreports.AuditQuery, string, error) {
	resp, err := c.grpcClient.ListAuditQueries(ctx, &pb.ListAuditQueriesRequest{
		PageSize:  int32(size),
		PageToken: token,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out, err := v1.FromProtoAuditQueries(resp.Queries)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return out, resp.GetNextPageToken(), nil

}

func (c *Client) GetSecurityReports(ctx context.Context) ([]*secreports.Report, error) {
	var resources []*pb.Report
	nextKey := ""
	for {
		resp, err := c.grpcClient.ListReports(ctx, &pb.ListReportsRequest{
			PageSize:  0,
			PageToken: nextKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, resp.GetReports()...)
		if resp.GetNextPageToken() == "" {
			break
		}
		nextKey = resp.GetNextPageToken()
	}
	out := make([]*secreports.Report, 0, len(resources))
	for _, v := range resources {
		item, err := v1.FromProtoReport(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, nil
}

func (c *Client) ListSecurityReports(ctx context.Context, pageSize int, token string) ([]*secreports.Report, string, error) {
	resp, err := c.grpcClient.ListReports(ctx, &pb.ListReportsRequest{
		PageSize:  int32(pageSize),
		PageToken: token,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	reports, err := v1.FromProtoReports(resp.Reports)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return reports, resp.GetNextPageToken(), nil
}

// GetSecurityReportExecutionState returns the execution state of the report.
func (c *Client) GetSecurityReportExecutionState(ctx context.Context, name string, days int32) (*secreports.ReportState, error) {
	resp, err := c.grpcClient.GetReportState(ctx, &pb.GetReportStateRequest{
		Name: name,
		Days: uint32(days),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	out, err := v1.FromProtoReportState(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// NewClient creates a new SecReports client.
func NewClient(grpcClient pb.SecReportsServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetSchema returns the schema for the audit query language.
func (c *Client) GetSchema(ctx context.Context) (*pb.GetSchemaResponse, error) {
	resp, err := c.grpcClient.GetSchema(ctx, &pb.GetSchemaRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// RunAuditQueryAndGetResult runs an audit query and returns the result.
func (c *Client) RunAuditQueryAndGetResult(ctx context.Context, queryText string, days int) ([]*pb.QueryRowResult, error) {
	resp, err := c.RunAuditQuery(ctx, queryText, days)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rows, err := c.GetAuditQueryResultAll(ctx, resp.GetResultId())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rows, nil
}

// RunAuditQuery runs an audit query.
func (c *Client) RunAuditQuery(ctx context.Context, queryText string, days int) (*pb.RunAuditQueryResponse, error) {
	resp, err := c.grpcClient.RunAuditQuery(ctx, &pb.RunAuditQueryRequest{
		Query: queryText,
		Days:  int32(days),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetAuditQueryResultAll returns all results for an audit query.
func (c *Client) GetAuditQueryResultAll(ctx context.Context, queryID string) ([]*pb.QueryRowResult, error) {
	var out []*pb.QueryRowResult
	var nextToken string
	for {
		resp, err := c.grpcClient.GetAuditQueryResult(ctx, &pb.GetAuditQueryResultRequest{
			ResultId:  queryID,
			NextToken: nextToken,
		})
		if err != nil {
			return nil, trail.FromGRPC(err)
		}
		out = append(out, resp.Result.GetRows()...)
		if resp.GetNextToken() == "" {
			break
		}
		nextToken = resp.GetNextToken()
	}
	return out, nil
}

// DeleteSecurityReport  deletes a security report.
func (c *Client) DeleteSecurityReport(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteReport(ctx, &pb.DeleteReportRequest{
		Name: name,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}
