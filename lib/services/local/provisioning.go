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

// ProvisioningService governs adding new nodes to the cluster
type ProvisioningService struct {
	backend.Backend
}

// NewProvisioningService returns a new instance of provisioning service
func NewProvisioningService(backend backend.Backend) *ProvisioningService {
	return &ProvisioningService{Backend: backend}
}

// UpsertToken adds provisioning tokens for the auth server
func (s *ProvisioningService) UpsertToken(ctx context.Context, p types.ProvisionToken) error {
	item, err := s.tokenToItem(p)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(ctx, *item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateToken creates a new token for the auth server
func (s *ProvisioningService) CreateToken(ctx context.Context, p types.ProvisionToken) error {
	item, err := s.tokenToItem(p)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Create(ctx, *item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *ProvisioningService) tokenToItem(p types.ProvisionToken) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(p); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := p.GetRevision()
	data, err := services.MarshalProvisionToken(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(tokensPrefix, p.GetName()),
		Value:    data,
		Expires:  p.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// DeleteAllTokens deletes all provisioning tokens
func (s *ProvisioningService) DeleteAllTokens() error {
	startKey := backend.NewKey(tokensPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// GetToken finds and returns token by ID
func (s *ProvisioningService) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	if token == "" {
		return nil, trace.BadParameter("missing parameter token")
	}
	item, err := s.Get(ctx, backend.NewKey(tokensPrefix, token))
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("provisioning token(%s) not found", backend.MaskKeyName(token))
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return services.UnmarshalProvisionToken(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// DeleteToken deletes a token by ID
func (s *ProvisioningService) DeleteToken(ctx context.Context, token string) error {
	if token == "" {
		return trace.BadParameter("missing parameter token")
	}
	err := s.Delete(ctx, backend.NewKey(tokensPrefix, token))
	if trace.IsNotFound(err) {
		return trace.NotFound("provisioning token(%s) not found", backend.MaskKeyName(token))
	}
	return trace.Wrap(err)
}

// GetTokens returns all active (non-expired) provisioning tokens
func (s *ProvisioningService) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	startKey := backend.ExactKey(tokensPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokens := make([]types.ProvisionToken, len(result.Items))
	for i, item := range result.Items {
		t, err := services.UnmarshalProvisionToken(
			item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens[i] = t
	}
	return tokens, nil
}

const tokensPrefix = "tokens"
