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
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// Provisioner manages nodes and tokens on the auth server
type Provisioner interface {
	auth.Provisioner

	// DeleteAllTokens deletes all provisioning tokens
	DeleteAllTokens() error
}

// ProvisioningService governs adding new nodes to the cluster
type ProvisioningService struct {
	backend.Backend
}

// NewProvisioningService returns a new instance of provisioning service
func NewProvisioningService(backend backend.Backend) *ProvisioningService {
	return &ProvisioningService{Backend: backend}
}

// UpsertToken adds provisioning tokens for the auth server
func (s *ProvisioningService) UpsertToken(ctx context.Context, p services.ProvisionToken) error {
	if err := p.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if p.Expiry().IsZero() || p.Expiry().Sub(s.Clock().Now().UTC()) < time.Second {
		p.SetExpiry(s.Clock().Now().UTC().Add(defaults.ProvisioningTokenTTL))
	}
	data, err := resource.MarshalProvisionToken(p)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(tokensPrefix, p.GetName()),
		Value:   data,
		Expires: p.Expiry(),
		ID:      p.GetResourceID(),
	}
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllTokens deletes all provisioning tokens
func (s *ProvisioningService) DeleteAllTokens() error {
	startKey := backend.Key(tokensPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// GetToken finds and returns token by ID
func (s *ProvisioningService) GetToken(ctx context.Context, token string) (services.ProvisionToken, error) {
	if token == "" {
		return nil, trace.BadParameter("missing parameter token")
	}
	item, err := s.Get(ctx, backend.Key(tokensPrefix, token))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resource.UnmarshalProvisionToken(item.Value, resource.SkipValidation(),
		resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
}

func (s *ProvisioningService) DeleteToken(ctx context.Context, token string) error {
	if token == "" {
		return trace.BadParameter("missing parameter token")
	}
	err := s.Delete(ctx, backend.Key(tokensPrefix, token))
	return trace.Wrap(err)
}

// GetTokens returns all active (non-expired) provisioning tokens
func (s *ProvisioningService) GetTokens(ctx context.Context, opts ...auth.MarshalOption) ([]services.ProvisionToken, error) {
	startKey := backend.Key(tokensPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokens := make([]services.ProvisionToken, len(result.Items))
	for i, item := range result.Items {
		t, err := resource.UnmarshalProvisionToken(item.Value,
			resource.AddOptions(opts, resource.SkipValidation(),
				resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens[i] = t
	}
	return tokens, nil
}

const tokensPrefix = "tokens"
