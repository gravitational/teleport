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

	"github.com/gravitational/trace"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

type CrownJewelsService struct {
	service *generic.ServiceWrapper[*crownjewelv1.CrownJewel]
}

const crownJewelsKey = "crown_jewels"

// NewCrownJewelsService creates a new CrownJewelsService.
func NewCrownJewelsService(b backend.Backend) (*CrownJewelsService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*crownjewelv1.CrownJewel]{
			Backend:       b,
			ResourceKind:  types.KindCrownJewel,
			BackendPrefix: backend.NewKey(crownJewelsKey),
			MarshalFunc:   services.MarshalCrownJewel,
			UnmarshalFunc: services.UnmarshalCrownJewel,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CrownJewelsService{service: service}, nil
}

func (s *CrownJewelsService) ListCrownJewels(ctx context.Context, pagesize int64, lastKey string) ([]*crownjewelv1.CrownJewel, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, int(pagesize), lastKey)
	return r, nextToken, trace.Wrap(err)
}

func (s *CrownJewelsService) GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error) {
	r, err := s.service.GetResource(ctx, name)
	return r, trace.Wrap(err)
}

func (s *CrownJewelsService) CreateCrownJewel(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	r, err := s.service.CreateResource(ctx, crownJewel)
	return r, trace.Wrap(err)
}

func (s *CrownJewelsService) UpdateCrownJewel(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, crownJewel)
	return r, trace.Wrap(err)
}

func (s *CrownJewelsService) UpsertCrownJewel(ctx context.Context, crownJewel *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	r, err := s.service.UpsertResource(ctx, crownJewel)
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
