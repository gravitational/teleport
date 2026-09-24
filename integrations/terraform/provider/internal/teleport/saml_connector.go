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
