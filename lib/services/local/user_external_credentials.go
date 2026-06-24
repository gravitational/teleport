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
	"iter"
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

// IterateUserExternalCredentials returns an iterator over user external
// credentials, filtered by the request parameters.
func (s *UserExternalCredentialsService) IterateUserExternalCredentials(ctx context.Context, req services.IterateUserExternalCredentialsRequest) iter.Seq2[*userexternalcredentialsv1.UserExternalCredentials, error] {
	var startKey backend.Key
	if req.User != "" {
		startKey = backend.NewKey(userExternalCredentialsPrefix, req.User)
	} else {
		startKey = backend.NewKey(userExternalCredentialsPrefix)
	}

	var keySuffix backend.Key
	if req.Name != "" {
		keySuffix = backend.NewKey(req.Name)
	}

	items := s.backend.Items(ctx, backend.ItemsParams{
		StartKey: startKey,
		EndKey:   backend.RangeEnd(startKey),
	})
	return func(yield func(*userexternalcredentialsv1.UserExternalCredentials, error) bool) {
		for item, err := range items {
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			if keySuffix.String() != "" && !item.Key.HasSuffix(keySuffix) {
				continue
			}
			creds, err := services.UnmarshalProtoResource[*userexternalcredentialsv1.UserExternalCredentials](item.Value)
			if err != nil {
				slog.WarnContext(ctx, "Failed to unmarshal user external credentials", "key", item.Key.String(), "error", err)
				continue
			}
			if !yield(creds, nil) {
				return
			}
		}
	}
}
