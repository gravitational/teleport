/*
Copyright 2016 Gravitational, Inc.

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
	"sort"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// AccessService manages roles
type AccessService struct {
	backend.Backend
}

// NewAccessService returns new access service instance
func NewAccessService(backend backend.Backend) *AccessService {
	return &AccessService{Backend: backend}
}

// GetRoles returns a list of roles registered with the local auth server
func (s *AccessService) GetRoles() ([]services.Role, error) {
	keys, err := s.GetKeys([]string{"roles"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []services.Role
	for _, name := range keys {
		u, err := s.GetRole(name)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		out = append(out, u)
	}
	sort.Sort(services.SortedRoles(out))
	return out, nil
}

// UpsertRole updates parameters about role
func (s *AccessService) UpsertRole(role services.Role, ttl time.Duration) error {
	data, err := services.GetRoleMarshaler().MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.UpsertVal([]string{"roles", role.GetName()}, "params", []byte(data), ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetRole returns a role by name
func (s *AccessService) GetRole(name string) (services.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing role name")
	}
	data, err := s.GetVal([]string{"roles", name}, "params")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("role %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return services.GetRoleMarshaler().UnmarshalRole(data)
}

// DeleteRole deletes a role with all the keys from the backend
func (s *AccessService) DeleteRole(role string) error {
	err := s.DeleteBucket([]string{"roles"}, role)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("role '%v' is not found", role)
		}
	}
	return trace.Wrap(err)
}
