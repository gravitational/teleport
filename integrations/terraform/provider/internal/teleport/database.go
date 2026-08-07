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

// NewDatabaseClient returns a database client.
func NewDatabaseClient(c *client.Client) DatabaseClient {
	return DatabaseClient{client: c}
}

// DatabaseClient manages database resources.
type DatabaseClient struct {
	client *client.Client
}

// Get reads a database by name.
func (r DatabaseClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.DatabaseV3, error) {
	database, err := r.client.GetDatabase(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databasev3, ok := database.(*types.DatabaseV3)
	if !ok {
		return nil, trace.BadParameter("unexpected database type: %T", database)
	}

	return databasev3, nil
}

// Create creates a database.
func (r DatabaseClient) Create(ctx context.Context, database *types.DatabaseV3) error {
	if err := r.client.CreateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a database.
func (r DatabaseClient) Upsert(ctx context.Context, database *types.DatabaseV3) error {
	if err := r.client.UpdateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a database by name.
func (r DatabaseClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteDatabase(ctx, id.Name))
}
