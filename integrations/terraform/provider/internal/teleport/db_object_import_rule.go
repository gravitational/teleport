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
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewDatabaseObjectImportRuleClient returns a database object import rule client.
func NewDatabaseObjectImportRuleClient(c *client.Client) DatabaseObjectImportRuleClient {
	return DatabaseObjectImportRuleClient{client: c}
}

// DatabaseObjectImportRuleClient manages database object import rule resources.
type DatabaseObjectImportRuleClient struct {
	client *client.Client
}

// Get reads a database object import rule by name.
func (r DatabaseObjectImportRuleClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*dbobjectimportrulev1.DatabaseObjectImportRule, error) {
	importRule, err := r.client.DatabaseObjectImportRuleClient().GetDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.GetDatabaseObjectImportRuleRequest{
		Name: id.Name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return importRule, nil
}

// Create creates a database object import rule.
func (r DatabaseObjectImportRuleClient) Create(ctx context.Context, importRule *dbobjectimportrulev1.DatabaseObjectImportRule) error {
	if _, err := r.client.DatabaseObjectImportRuleClient().CreateDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.CreateDatabaseObjectImportRuleRequest{
		Rule: importRule,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a database object import rule.
func (r DatabaseObjectImportRuleClient) Upsert(ctx context.Context, importRule *dbobjectimportrulev1.DatabaseObjectImportRule) error {
	if _, err := r.client.DatabaseObjectImportRuleClient().UpsertDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.UpsertDatabaseObjectImportRuleRequest{
		Rule: importRule,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a database object import rule by name.
func (r DatabaseObjectImportRuleClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	_, err := r.client.DatabaseObjectImportRuleClient().DeleteDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.DeleteDatabaseObjectImportRuleRequest{
		Name: id.Name,
	})
	return trace.Wrap(err)
}
