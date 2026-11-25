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
