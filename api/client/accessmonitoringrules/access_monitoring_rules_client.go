// Copyright 2024 Gravitational, Inc.
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

package accessmonitoringrules

import (
	"context"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
)

// Client is an access monitoring rules client that conforms to services.AccessMonitoringRules.
type Client struct {
	grpcClient accessmonitoringrulesv1.AccessMonitoringRulesServiceClient
}

// NewClient returns and access monitoring rules client
func NewClient(grpcClient accessmonitoringrulesv1.AccessMonitoringRulesServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// CreateAccessMonitoringRule creates the specified access monitoring rule.
func (c *Client) CreateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	req := &accessmonitoringrulesv1.CreateAccessMonitoringRuleRequest{
		Rule: in,
	}
	resp, err := c.grpcClient.CreateAccessMonitoringRule(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpdateAccessMonitoringRule updates the specified access monitoring rule.
func (c *Client) UpdateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	req := &accessmonitoringrulesv1.UpdateAccessMonitoringRuleRequest{
		Rule: in,
	}
	resp, err := c.grpcClient.UpdateAccessMonitoringRule(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpsertAccessMonitoringRule upserts the specified access monitoring rule.
func (c *Client) UpsertAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	req := &accessmonitoringrulesv1.UpsertAccessMonitoringRuleRequest{
		Rule: in,
	}
	resp, err := c.grpcClient.UpsertAccessMonitoringRule(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// GetAccessMonitoringRule gets the specified access monitoring rule.
func (c *Client) GetAccessMonitoringRule(ctx context.Context, resourceName string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	req := &accessmonitoringrulesv1.GetAccessMonitoringRuleRequest{
		Name: resourceName,
	}
	resp, err := c.grpcClient.GetAccessMonitoringRule(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// DeleteAccessMonitoringRule deletes the specified access monitoring rule.
func (c *Client) DeleteAccessMonitoringRule(ctx context.Context, resourceName string) error {
	req := &accessmonitoringrulesv1.DeleteAccessMonitoringRuleRequest{
		Name: resourceName,
	}
	_, err := c.grpcClient.DeleteAccessMonitoringRule(ctx, req)
	return trace.Wrap(err)
}

// DeleteAllAccessMonitoringRules deletes all access monitoring rules.
func (c *Client) DeleteAllAccessMonitoringRules(ctx context.Context) error {
	req := &accessmonitoringrulesv1.DeleteAccessMonitoringRuleRequest{}
	_, err := c.grpcClient.DeleteAccessMonitoringRule(ctx, req)
	return trace.Wrap(err)
}

// ListAccessMonitoringRules lists current access monitoring rules.
func (c *Client) ListAccessMonitoringRules(ctx context.Context, pageSize int, pageToken string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	resp, err := c.grpcClient.ListAccessMonitoringRules(ctx, &accessmonitoringrulesv1.ListAccessMonitoringRulesRequest{
		PageSize:  int64(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp.Rules, resp.GetNextPageToken(), nil
}

// ListAccessMonitoringRulesWithFilter lists current access monitoring rules.
func (c *Client) ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	resp, err := c.grpcClient.ListAccessMonitoringRulesWithFilter(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp.Rules, resp.GetNextPageToken(), nil
}
