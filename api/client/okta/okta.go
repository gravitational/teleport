// Copyright 2023 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package okta

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/protobuf/types/known/durationpb"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
)

// Client is an Okta client that conforms to the following lib/services interfaces:
// * services.OktaImportRules
// * services.OktaAssignments
type Client struct {
	grpcClient oktapb.OktaServiceClient
}

// NewClient creates a new Okta client.
func NewClient(grpcClient oktapb.OktaServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListOktaImportRules returns a paginated list of all Okta import rule resources.
func (c *Client) ListOktaImportRules(ctx context.Context, pageSize int, pageToken string) ([]types.OktaImportRule, string, error) {
	resp, err := c.grpcClient.ListOktaImportRules(ctx, &oktapb.ListOktaImportRulesRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	importRules := make([]types.OktaImportRule, len(resp.ImportRules))
	for i, importRule := range resp.ImportRules {
		importRules[i] = importRule
	}

	return importRules, resp.NextPageToken, nil
}

// GetOktaImportRule returns the specified Okta import rule resources.
func (c *Client) GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error) {
	resp, err := c.grpcClient.GetOktaImportRule(ctx, &oktapb.GetOktaImportRuleRequest{
		Name: name,
	})
	return resp, trail.FromGRPC(err)
}

// CreateOktaImportRule creates a new Okta import rule resource.
func (c *Client) CreateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	importRuleV1, ok := importRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("import rule expected to be OktaImportRuleV1, got %T", importRule)
	}
	resp, err := c.grpcClient.CreateOktaImportRule(ctx, &oktapb.CreateOktaImportRuleRequest{
		ImportRule: importRuleV1,
	})
	return resp, trail.FromGRPC(err)
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (c *Client) UpdateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	importRuleV1, ok := importRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("import rule expected to be OktaImportRuleV1, got %T", importRule)
	}
	resp, err := c.grpcClient.UpdateOktaImportRule(ctx, &oktapb.UpdateOktaImportRuleRequest{
		ImportRule: importRuleV1,
	})
	return resp, trail.FromGRPC(err)
}

// DeleteOktaImportRule removes the specified Okta import rule resource.
func (c *Client) DeleteOktaImportRule(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteOktaImportRule(ctx, &oktapb.DeleteOktaImportRuleRequest{
		Name: name,
	})
	return trail.FromGRPC(err)
}

// DeleteAllOktaImportRules removes all Okta import rules.
func (c *Client) DeleteAllOktaImportRules(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAllOktaImportRules(ctx, &oktapb.DeleteAllOktaImportRulesRequest{})
	return trail.FromGRPC(err)
}

// ListOktaAssignments returns a paginated list of all Okta assignment resources.
func (c *Client) ListOktaAssignments(ctx context.Context, pageSize int, pageToken string) ([]types.OktaAssignment, string, error) {
	resp, err := c.grpcClient.ListOktaAssignments(ctx, &oktapb.ListOktaAssignmentsRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	assignments := make([]types.OktaAssignment, len(resp.Assignments))
	for i, assignment := range resp.Assignments {
		assignments[i] = assignment
	}

	return assignments, resp.NextPageToken, nil
}

// GetOktaAssignmentreturns the specified Okta assignment resources.
func (c *Client) GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error) {
	resp, err := c.grpcClient.GetOktaAssignment(ctx, &oktapb.GetOktaAssignmentRequest{
		Name: name,
	})
	return resp, trail.FromGRPC(err)
}

// CreateOktaAssignmentcreates a new Okta assignment resource.
func (c *Client) CreateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	assignmentV1, ok := assignment.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("import rule expected to be OktaAssignmentV1, got %T", assignment)
	}
	resp, err := c.grpcClient.CreateOktaAssignment(ctx, &oktapb.CreateOktaAssignmentRequest{
		Assignment: assignmentV1,
	})
	return resp, trail.FromGRPC(err)
}

// UpdateOktaAssignmentupdates an existing Okta assignment resource.
func (c *Client) UpdateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	assignmentV1, ok := assignment.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("import rule expected to be OktaAssignmentV1, got %T", assignment)
	}
	resp, err := c.grpcClient.UpdateOktaAssignment(ctx, &oktapb.UpdateOktaAssignmentRequest{
		Assignment: assignmentV1,
	})
	return resp, trail.FromGRPC(err)
}

// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
// since the last transition.
func (c *Client) UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error {
	_, err := c.grpcClient.UpdateOktaAssignmentStatus(ctx, &oktapb.UpdateOktaAssignmentStatusRequest{
		Name:          name,
		Status:        types.OktaAssignmentStatusToProto(status),
		TimeHasPassed: durationpb.New(timeHasPassed),
	})
	return trail.FromGRPC(err)
}

// DeleteOktaAssignmentremoves the specified Okta assignment resource.
func (c *Client) DeleteOktaAssignment(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteOktaAssignment(ctx, &oktapb.DeleteOktaAssignmentRequest{
		Name: name,
	})
	return trail.FromGRPC(err)
}

// DeleteAllOktaAssignments removes all Okta assignments.
func (c *Client) DeleteAllOktaAssignments(ctx context.Context) error {
	_, err := c.grpcClient.DeleteAllOktaAssignments(ctx, &oktapb.DeleteAllOktaAssignmentsRequest{})
	return trail.FromGRPC(err)
}
