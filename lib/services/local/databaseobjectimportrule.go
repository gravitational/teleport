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

package local

import (
	"context"

	"github.com/gravitational/trace"

	databaseobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// databaseObjectImportRuleService manages database object import rules in the backend.
type databaseObjectImportRuleService struct {
	service *generic.ServiceWrapper[*databaseobjectimportrulev1.DatabaseObjectImportRule]
}

var _ services.DatabaseObjectImportRules = (*databaseObjectImportRuleService)(nil)

func (s *databaseObjectImportRuleService) UpsertDatabaseObjectImportRule(ctx context.Context, rule *databaseobjectimportrulev1.DatabaseObjectImportRule) (*databaseobjectimportrulev1.DatabaseObjectImportRule, error) {
	out, err := s.service.UpsertResource(ctx, rule)
	return out, trace.Wrap(err)
}

func (s *databaseObjectImportRuleService) UpdateDatabaseObjectImportRule(ctx context.Context, rule *databaseobjectimportrulev1.DatabaseObjectImportRule) (*databaseobjectimportrulev1.DatabaseObjectImportRule, error) {
	out, err := s.service.UnconditionalUpdateResource(ctx, rule)
	return out, trace.Wrap(err)
}

func (s *databaseObjectImportRuleService) CreateDatabaseObjectImportRule(ctx context.Context, rule *databaseobjectimportrulev1.DatabaseObjectImportRule) (*databaseobjectimportrulev1.DatabaseObjectImportRule, error) {
	out, err := s.service.CreateResource(ctx, rule)
	return out, trace.Wrap(err)
}

func (s *databaseObjectImportRuleService) GetDatabaseObjectImportRule(ctx context.Context, name string) (*databaseobjectimportrulev1.DatabaseObjectImportRule, error) {
	out, err := s.service.GetResource(ctx, name)
	return out, trace.Wrap(err)
}

func (s *databaseObjectImportRuleService) DeleteDatabaseObjectImportRule(ctx context.Context, name string) error {
	return trace.Wrap(s.service.DeleteResource(ctx, name))
}

func (s *databaseObjectImportRuleService) ListDatabaseObjectImportRules(ctx context.Context, size int, pageToken string) ([]*databaseobjectimportrulev1.DatabaseObjectImportRule, string, error) {
	out, next, err := s.service.ListResources(ctx, size, pageToken)
	return out, next, trace.Wrap(err)
}

const (
	databaseObjectImportRulePrefix = "databaseObjectImportRulePrefix"
)

func NewDatabaseObjectImportRuleService(b backend.Backend) (services.DatabaseObjectImportRules, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*databaseobjectimportrulev1.DatabaseObjectImportRule]{
			Backend:       b,
			ResourceKind:  types.KindDatabaseObjectImportRule,
			BackendPrefix: backend.NewKey(databaseObjectImportRulePrefix),
			//nolint:staticcheck // SA1019. Using this marshaler for json compatibility.
			MarshalFunc: services.FastMarshalProtoResourceDeprecated[*databaseobjectimportrulev1.DatabaseObjectImportRule],
			//nolint:staticcheck // SA1019. Using this unmarshaler for json compatibility.
			UnmarshalFunc: services.FastUnmarshalProtoResourceDeprecated[*databaseobjectimportrulev1.DatabaseObjectImportRule],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &databaseObjectImportRuleService{service: service}, nil
}
