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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

type CrownJewelsService struct {
	backend.Backend
}

// NewCrownJewelsService creates a new CrownJewelsService.
func NewCrownJewelsService(backend backend.Backend) *CrownJewelsService {
	return &CrownJewelsService{Backend: backend}
}

const (
	crownJewelsKey = "crown_jewels"
)

func (s *CrownJewelsService) GetCrownJewels(ctx context.Context) ([]*types.CrownJewel, error) {
	startKey := backend.ExactKey(crownJewelsKey)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	crownJewels := make([]*types.CrownJewel, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalCrownJewel(item.Value,
			services.WithResourceID(item.ID),
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		crownJewels[i] = cluster
	}
	return crownJewels, nil
}

func (s *CrownJewelsService) CreateCrownJewel(ctx context.Context, cluster *types.CrownJewel) (*types.CrownJewel, error) {
	if err := services.CheckAndSetDefaults(cluster); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalCrownJewel(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(crownJewelsKey, cluster.GetName()),
		Value:   value,
		Expires: cluster.Expiry(),
		ID:      cluster.GetResourceID(),
	}
	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster.SetResourceID(lease.ID)
	cluster.SetRevision(lease.Revision)
	return cluster, nil
}

func (s *CrownJewelsService) DeleteCrownJewel(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(crownJewelsKey, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("crown jewel %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (s *CrownJewelsService) DeleteAllCrownJewels(ctx context.Context) error {
	startKey := backend.ExactKey(crownJewelsKey)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
