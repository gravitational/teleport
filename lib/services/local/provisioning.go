/*
Copyright 2015 Gravitational, Inc.

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
	"encoding/json"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
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
func (s *ProvisioningService) UpsertToken(token string, roles teleport.Roles, ttl time.Duration) error {
	if ttl < time.Second {
		ttl = defaults.ProvisioningTokenTTL
	}
	t := services.ProvisionToken{
		Roles:   roles,
		Expires: time.Now().UTC().Add(ttl),
		Token:   token,
	}
	value, err := json.Marshal(t)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(tokensPrefix, token),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(ttl),
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetToken finds and returns token by id
func (s *ProvisioningService) GetToken(token string) (*services.ProvisionToken, error) {
	if token == "" {
		return nil, trace.BadParameter("missing parameter token")
	}
	item, err := s.Get(context.TODO(), backend.Key(tokensPrefix, token))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var t services.ProvisionToken
	err = json.Unmarshal(item.Value, &t)
	if err != nil {
		t.Token = token // for backwards compatibility with older tokens
		return nil, trace.Wrap(err)
	}
	return &t, nil
}

func (s *ProvisioningService) DeleteToken(token string) error {
	if token == "" {
		return trace.BadParameter("missing parameter token")
	}
	err := s.Delete(context.TODO(), backend.Key(tokensPrefix, token))
	return trace.Wrap(err)
}

// GetTokens returns all active (non-expired) provisioning tokens
func (s *ProvisioningService) GetTokens() ([]services.ProvisionToken, error) {
	startKey := backend.Key(tokensPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokens := make([]services.ProvisionToken, len(result.Items))
	for i, item := range result.Items {
		var t services.ProvisionToken
		err = json.Unmarshal(item.Value, &t)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens[i] = t
	}
	return tokens, nil
}

const tokensPrefix = "tokens"
