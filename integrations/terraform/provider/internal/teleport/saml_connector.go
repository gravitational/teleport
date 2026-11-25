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

// NewSAMLConnectorClient returns a new SAML connector client.
func NewSAMLConnectorClient(c *client.Client) SAMLConnectorClient {
	return SAMLConnectorClient{client: c}
}

// SAMLConnectorClient manages saml connectors.
type SAMLConnectorClient struct {
	client *client.Client
}

// Get reads a SAML connector by name.
func (c SAMLConnectorClient) Get(
	ctx context.Context,
	id tfdriver.NameIdentifier,
) (*types.SAMLConnectorV2, error) {
	connector, err := c.client.GetSAMLConnectorWithValidationOptions(
		ctx,
		id.Name,
		true,
		types.SAMLConnectorValidationFollowURLs(false),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorV2, ok := connector.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trace.BadParameter(
			"unexpected SAML connector type: %T",
			connector,
		)
	}
	return connectorV2, nil
}

// Create creates a SAML Connector.
func (c SAMLConnectorClient) Create(ctx context.Context, sc *types.SAMLConnectorV2) error {
	_, err := c.client.CreateSAMLConnector(ctx, sc)
	return trace.Wrap(err)
}

// Upsert updates a SAML Connector.
func (c SAMLConnectorClient) Upsert(ctx context.Context, sc *types.SAMLConnectorV2) error {
	_, err := c.client.UpsertSAMLConnector(ctx, sc)
	return trace.Wrap(err)
}

// Delete deletes a SAML Connector.
func (c SAMLConnectorClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(c.client.DeleteSAMLConnector(ctx, id.Name))
}
