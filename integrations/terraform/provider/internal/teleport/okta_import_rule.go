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
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewOktaImportRuleClient returns an Okta import rule client.
func NewOktaImportRuleClient(c *client.Client) OktaImportRuleClient {
	return OktaImportRuleClient{client: c}
}

// OktaImportRuleClient manages Okta import rule resources.
type OktaImportRuleClient struct {
	client *client.Client
}

// Get reads an Okta import rule by name.
func (r OktaImportRuleClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.OktaImportRuleV1, error) {
	importRule, err := r.client.OktaClient().GetOktaImportRule(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	importRuleV1, ok := importRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Okta import rule type: %T", importRule)
	}

	return importRuleV1, nil
}

// Create creates an Okta import rule.
func (r OktaImportRuleClient) Create(ctx context.Context, importRule *types.OktaImportRuleV1) error {
	if _, err := r.client.OktaClient().CreateOktaImportRule(ctx, importRule); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an Okta import rule.
func (r OktaImportRuleClient) Upsert(ctx context.Context, importRule *types.OktaImportRuleV1) error {
	if _, err := r.client.OktaClient().UpdateOktaImportRule(ctx, importRule); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an Okta import rule by name.
func (r OktaImportRuleClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.OktaClient().DeleteOktaImportRule(ctx, id.Name))
}
