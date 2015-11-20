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
package services

import (
	"encoding/json"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/backend"
)

type ProvisioningService struct {
	backend backend.Backend
}

func NewProvisioningService(backend backend.Backend) *ProvisioningService {
	return &ProvisioningService{backend}
}

// Tokens are provisioning tokens for the auth server
func (s *ProvisioningService) UpsertToken(token, domainName, role string, ttl time.Duration) error {
	t := ProvisionToken{
		DomainName: domainName,
		Role: role,
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
func (s *ProvisioningService) GetToken(token string) (ProvisionToken, error) {
	out, err := s.backend.GetVal([]string{"tokens"}, token)
	if err != nil {
		return ProvisionToken{}, err
	}
	var t ProvisionToken
	err = json.Unmarshal(out, &t)
	if err != nil {
		return ProvisionToken{}, trace.Wrap(err)
	}

	return t, nil
}
func (s *ProvisioningService) DeleteToken(token string) error {
	err := s.backend.DeleteKey([]string{"tokens"}, token)
	return err
}

type ProvisionToken struct {
	DomainName string
	Role string
}
