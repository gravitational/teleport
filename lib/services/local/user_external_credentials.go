/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	userExternalCredentialsPrefix = "user_external_credentials"
	githubOAuthPrefix             = "github_oauth"
)

// UserExternalCredentialsService manages per-user credentials for external services.
type UserExternalCredentialsService struct {
	backend backend.Backend
}

// NewUserExternalCredentialsService returns a new UserExternalCredentialsService.
func NewUserExternalCredentialsService(b backend.Backend) *UserExternalCredentialsService {
	return &UserExternalCredentialsService{backend: b}
}

func (s *UserExternalCredentialsService) backendKey(user, name string) backend.Key {
	return backend.NewKey(userExternalCredentialsPrefix, user, githubOAuthPrefix, name)
}

// GetUserExternalCredentials gets a UserExternalCredentials resource by user and name.
func (s *UserExternalCredentialsService) GetUserExternalCredentials(ctx context.Context, user, name string) (*userexternalcredentialsv1.UserExternalCredentials, error) {
	item, err := s.backend.Get(ctx, s.backendKey(user, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := services.UnmarshalProtoResource[*userexternalcredentialsv1.UserExternalCredentials](item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// UpsertUserExternalCredentials creates or updates a UserExternalCredentials resource.
func (s *UserExternalCredentialsService) UpsertUserExternalCredentials(ctx context.Context, creds *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error) {
	value, err := services.MarshalProtoResource[*userexternalcredentialsv1.UserExternalCredentials](creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user := creds.GetSpec().GetUser()
	name := creds.GetMetadata().GetName()
	item := backend.Item{
		Key:   s.backendKey(user, name),
		Value: value,
	}
	if expires := creds.GetMetadata().GetExpires(); expires != nil && expires.IsValid() {
		expiry := expires.AsTime()
		if !expiry.IsZero() && expiry.After(time.Now()) {
			item.Expires = expiry
		}
	}
	slog.DebugContext(ctx, "Upserting user external credentials",
		"key", item.Key.String(),
		"user", user,
		"name", name,
		"value_len", len(item.Value),
	)
	_, err = s.backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Successfully upserted user external credentials",
		"key", item.Key.String(),
	)
	return creds, nil
}

// DeleteUserExternalCredentials deletes a UserExternalCredentials resource by user and name.
func (s *UserExternalCredentialsService) DeleteUserExternalCredentials(ctx context.Context, user, name string) error {
	return trace.Wrap(s.backend.Delete(ctx, s.backendKey(user, name)))
}
