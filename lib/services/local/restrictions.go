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

// RestrictionsService manages restrictions to be enforced by restricted shell
type RestrictionsService struct {
	backend.Backend
}

// NewRestrictionsService creates a new RestrictionsService
func NewRestrictionsService(backend backend.Backend) *RestrictionsService {
	return &RestrictionsService{Backend: backend}
}

// SetNetworkRestrictions upserts NetworkRestrictions
func (s *RestrictionsService) SetNetworkRestrictions(ctx context.Context, nr types.NetworkRestrictions) error {
	if err := services.CheckAndSetDefaults(nr); err != nil {
		return trace.Wrap(err)
	}
	rev := nr.GetRevision()
	value, err := services.MarshalNetworkRestrictions(nr)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.Key(restrictionsPrefix, network),
		Value:    value,
		Expires:  nr.Expiry(),
		ID:       nr.GetResourceID(),
		Revision: rev,
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *RestrictionsService) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	item, err := s.Get(context.TODO(), backend.Key(restrictionsPrefix, network))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalNetworkRestrictions(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// SetNetworkRestrictions upserts NetworkRestrictions
func (s *RestrictionsService) DeleteNetworkRestrictions(ctx context.Context) error {
	return trace.Wrap(s.Delete(ctx, backend.Key(restrictionsPrefix, network)))
}

const (
	restrictionsPrefix = "restrictions"
	network            = "network"
)
