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
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// ProvisioningService governs adding new nodes to the cluster
type ProvisioningService struct {
	backend backend.Backend
}

// NewProvisioningService returns a new instance of provisioning service
func NewProvisioningService(backend backend.Backend) *ProvisioningService {
	return &ProvisioningService{backend}
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
	out, err := json.Marshal(t)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal([]string{"tokens"}, token, out, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetToken finds and returns token by id
func (s *ProvisioningService) GetToken(token string) (*services.ProvisionToken, error) {
	out, err := s.backend.GetVal([]string{"tokens"}, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var t *services.ProvisionToken
	err = json.Unmarshal(out, &t)
	if err != nil {
		t.Token = token // for backwards compatibility with older tokens
		return nil, trace.Wrap(err)
	}
	return t, nil
}

func (s *ProvisioningService) DeleteToken(token string) error {
	err := s.backend.DeleteKey([]string{"tokens"}, token)
	return err
}

// GetTokens returns all active (non-expired) provisioning tokens
func (s *ProvisioningService) GetTokens() (tokens []services.ProvisionToken, err error) {
	keys, err := s.backend.GetKeys([]string{"tokens"})
	if err != nil {
		return tokens, trace.Wrap(err)
	}
	for _, k := range keys {
		tok, err := s.GetToken(k)
		if err != nil {
			log.Error(err)
		}
		tokens = append(tokens, *tok)
	}
	return tokens, nil
}
