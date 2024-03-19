/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// DatabaseObjectImportRules manages DatabaseObjectImportRule resources.
type DatabaseObjectImportRules interface {
	// CreateDatabaseObjectImportRule will create a new DatabaseObjectImportRule resource.
	CreateDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// UpsertDatabaseObjectImportRule creates a new DatabaseObjectImportRule or forcefully updates an existing DatabaseObjectImportRule.
	UpsertDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// GetDatabaseObjectImportRule will get a DatabaseObjectImportRule resource by name.
	GetDatabaseObjectImportRule(ctx context.Context, name string) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// DeleteDatabaseObjectImportRule will delete a DatabaseObjectImportRule resource.
	DeleteDatabaseObjectImportRule(ctx context.Context, name string) error

	// UpdateDatabaseObjectImportRule updates an existing DatabaseObjectImportRule.
	UpdateDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// ListDatabaseObjectImportRules will list DatabaseObjectImportRule resources.
	ListDatabaseObjectImportRules(ctx context.Context, size int, pageToken string) ([]*dbobjectimportrulev1.DatabaseObjectImportRule, string, error)
}

// MarshalDatabaseObjectImportRule marshals DatabaseObjectImportRule resource to JSON.
func MarshalDatabaseObjectImportRule(rule *dbobjectimportrulev1.DatabaseObjectImportRule, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		rule = proto.Clone(rule).(*dbobjectimportrulev1.DatabaseObjectImportRule)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		rule.Metadata.Id = 0
		rule.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalDatabaseObjectImportRule unmarshals the DatabaseObjectImportRule resource from JSON.
func UnmarshalDatabaseObjectImportRule(data []byte, opts ...MarshalOption) (*dbobjectimportrulev1.DatabaseObjectImportRule, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing DatabaseObjectImportRule data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj dbobjectimportrulev1.DatabaseObjectImportRule
	err = utils.FastUnmarshal(data, &obj)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Id is deprecated, but still needed.
		obj.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}
