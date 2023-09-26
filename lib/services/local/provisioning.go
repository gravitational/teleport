/*
Copyright 2015-2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	if err := p.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := services.MarshalProvisionToken(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(tokensPrefix, p.GetName()),
		Value:   data,
		Expires: p.Expiry(),
		ID:      p.GetResourceID(),
	}
	return item, nil
}

// DeleteAllTokens deletes all provisioning tokens
func (s *ProvisioningService) DeleteAllTokens() error {
	startKey := backend.Key(tokensPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// GetToken finds and returns token by ID
func (s *ProvisioningService) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	if token == "" {
		return nil, trace.BadParameter("missing parameter token")
	}
	item, err := s.Get(ctx, backend.Key(tokensPrefix, token))
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("provisioning token(%s) not found", backend.MaskKeyName(token))
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return services.UnmarshalProvisionToken(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// DeleteToken deletes a token by ID
func (s *ProvisioningService) DeleteToken(ctx context.Context, token string) error {
	if token == "" {
		return trace.BadParameter("missing parameter token")
	}
	err := s.Delete(ctx, backend.Key(tokensPrefix, token))
	if trace.IsNotFound(err) {
		return trace.NotFound("provisioning token(%s) not found", backend.MaskKeyName(token))
	}
	return trace.Wrap(err)
}

// GetTokens returns all active (non-expired) provisioning tokens
func (s *ProvisioningService) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	startKey := backend.Key(tokensPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokens := make([]types.ProvisionToken, len(result.Items))
	for i, item := range result.Items {
		t, err := services.UnmarshalProvisionToken(
			item.Value,
			services.WithResourceID(item.ID),
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
