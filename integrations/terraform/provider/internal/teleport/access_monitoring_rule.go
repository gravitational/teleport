// Copyright 2026 Gravitational, Inc.
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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewAccessMonitoringRuleClient returns an access monitoring rule client.
func NewAccessMonitoringRuleClient(c *client.Client) AccessMonitoringRuleClient {
	return AccessMonitoringRuleClient{client: c}
}

// AccessMonitoringRuleClient manages access monitoring rule resources.
type AccessMonitoringRuleClient struct {
	client *client.Client
}

// Get reads an access monitoring rule by name.
func (r AccessMonitoringRuleClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	accessMonitoringRule, err := r.client.AccessMonitoringRulesClient().GetAccessMonitoringRule(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessMonitoringRule, nil
}

// Create creates an access monitoring rule.
func (r AccessMonitoringRuleClient) Create(ctx context.Context, accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule) error {
	if _, err := r.client.AccessMonitoringRulesClient().CreateAccessMonitoringRule(ctx, accessMonitoringRule); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an access monitoring rule.
func (r AccessMonitoringRuleClient) Upsert(ctx context.Context, accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule) error {
	if _, err := r.client.AccessMonitoringRulesClient().UpdateAccessMonitoringRule(ctx, accessMonitoringRule); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an access monitoring rule by name.
func (r AccessMonitoringRuleClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, id.Name))
}
