// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
