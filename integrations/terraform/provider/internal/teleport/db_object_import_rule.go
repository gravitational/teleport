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
