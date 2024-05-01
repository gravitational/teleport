/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/golang/protobuf/proto"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
)

type CrownJewelsService struct {
	service *generic.ServiceWrapper[*crownjewelv1.CrownJewel]
}

const (
	crownJewelsKey = "crown_jewels"
)

// NewCrownJewelsService creates a new CrownJewelsService.
func NewCrownJewelsService(backend backend.Backend) (*CrownJewelsService, error) {
	service, err := generic.NewServiceWrapper(backend,
		types.KindCrownJewel,
		crownJewelsKey,
		MarshalCrownJewel,
		UnmarshalCrownJewel)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CrownJewelsService{service: service}, nil
}

func MarshalCrownJewel(object *crownjewelv1.CrownJewel, opts ...services.MarshalOption) ([]byte, error) {
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		object = proto.Clone(object).(*crownjewelv1.CrownJewel)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		object.Metadata.Id = 0
		object.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func UnmarshalCrownJewel(data []byte, opts ...services.MarshalOption) (*crownjewelv1.CrownJewel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing DatabaseObject data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj crownjewelv1.CrownJewel
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

func (s *CrownJewelsService) ListCrownJewels(ctx context.Context, pagesize int64, lastKey string) ([]*crownjewelv1.CrownJewel, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, int(pagesize), lastKey)
	return r, nextToken, trace.Wrap(err)
}

func (s *CrownJewelsService) CreateCrownJewel(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	r, err := s.service.CreateResource(ctx, crownJewel)
	return r, trace.Wrap(err)
}

func (s *CrownJewelsService) UpdateCrownJewel(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	r, err := s.service.UpdateResource(ctx, crownJewel)
	return r, trace.Wrap(err)
}

func (s *CrownJewelsService) DeleteCrownJewel(ctx context.Context, name string) error {
	err := s.service.DeleteResource(ctx, name)
	return trace.Wrap(err)
}

func (s *CrownJewelsService) DeleteAllCrownJewels(ctx context.Context) error {
	err := s.service.DeleteAllResources(ctx)
	return trace.Wrap(err)
}
