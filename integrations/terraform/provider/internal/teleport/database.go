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
