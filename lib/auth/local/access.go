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
	"context"
	"sort"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// Announcer is responsible for announcing presence on the auth server
type Announcer interface {
	auth.Announcer
	auth.KeepAliver
}

// Access manages roles and permissions on the auth server
type Access interface {
	auth.Access

	// CreateRole creates a role
	CreateRole(types.Role) error

	// DeleteAllRoles deletes all roles
	DeleteAllRoles() error
}

// AccessService manages roles
type AccessService struct {
	backend.Backend
}

// NewAccessService returns new access service instance
func NewAccessService(backend backend.Backend) *AccessService {
	return &AccessService{Backend: backend}
}

// DeleteAllRoles deletes all roles
func (s *AccessService) DeleteAllRoles() error {
	return s.DeleteRange(context.TODO(), backend.Key(rolesPrefix), backend.RangeEnd(backend.Key(rolesPrefix)))
}

// GetRoles returns a list of roles registered with the local auth server
func (s *AccessService) GetRoles(ctx context.Context) ([]services.Role, error) {
	result, err := s.GetRange(context.TODO(), backend.Key(rolesPrefix), backend.RangeEnd(backend.Key(rolesPrefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.Role, 0, len(result.Items))
	for _, item := range result.Items {
		role, err := resource.UnmarshalRole(item.Value,
			resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, role)
	}
	sort.Sort(auth.SortedRoles(out))
	return out, nil
}

// CreateRole creates a role on the backend.
func (s *AccessService) CreateRole(role services.Role) error {
	value, err := resource.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(rolesPrefix, role.GetName(), paramsPrefix),
		Value:   value,
		Expires: role.Expiry(),
	}

	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertRole updates parameters about role
func (s *AccessService) UpsertRole(ctx context.Context, role services.Role) error {
	value, err := resource.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(rolesPrefix, role.GetName(), paramsPrefix),
		Value:   value,
		Expires: role.Expiry(),
		ID:      role.GetResourceID(),
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetRole returns a role by name
func (s *AccessService) GetRole(ctx context.Context, name string) (services.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing role name")
	}
	item, err := s.Get(context.TODO(), backend.Key(rolesPrefix, name, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("role %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return resource.UnmarshalRole(item.Value,
		resource.WithResourceID(item.ID), resource.WithExpires(item.Expires))
}

// DeleteRole deletes a role from the backend
func (s *AccessService) DeleteRole(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing role name")
	}
	err := s.Delete(ctx, backend.Key(rolesPrefix, name, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("role %q is not found", name)
		}
	}
	return trace.Wrap(err)
}

const (
	rolesPrefix  = "roles"
	paramsPrefix = "params"
)

// WebSessionsGetter manages web sessions on auth server
type WebSessionsGetter interface {
	// WebSessions returns the web session manager
	WebSessions() WebSessions
}

// WebSessions manages web sessions on the auth server
type WebSessions interface {
	types.WebSessionInterface

	// Upsert updates existing or inserts a new web session.
	Upsert(ctx context.Context, session types.WebSession) error
}

// WebTokensGetter manages web tokens on auth server
type WebTokensGetter interface {
	// WebTokens returns the web token manager
	WebTokens() WebTokens
}

// WebTokens manages web session on the auth server
type WebTokens interface {
	types.WebTokenInterface

	// Upsert updates existing or inserts a new web token.
	Upsert(ctx context.Context, token types.WebToken) error
}

// AppSession manages application sessions on auth server
type AppSession interface {
	auth.AppSession

	// UpsertAppSession upserts and application web session.
	UpsertAppSession(context.Context, types.WebSession) error
}
