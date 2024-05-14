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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	databaseobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
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
	out, err := s.service.UpdateResource(ctx, rule)
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

func NewDatabaseObjectImportRuleService(backend backend.Backend) (services.DatabaseObjectImportRules, error) {
	service, err := generic.NewServiceWrapper(backend,
		types.KindDatabaseObjectImportRule,
		databaseObjectImportRulePrefix,
		marshalDatabaseObjectImportRule,
		unmarshalDatabaseObjectImportRule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &databaseObjectImportRuleService{service: service}, nil
}

func marshalDatabaseObjectImportRule(rule *databaseobjectimportrulev1.DatabaseObjectImportRule, opts ...services.MarshalOption) ([]byte, error) {
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		rule = proto.Clone(rule).(*databaseobjectimportrulev1.DatabaseObjectImportRule)
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

func unmarshalDatabaseObjectImportRule(data []byte, opts ...services.MarshalOption) (*databaseobjectimportrulev1.DatabaseObjectImportRule, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing DatabaseObjectImportRule data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj databaseobjectimportrulev1.DatabaseObjectImportRule
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
