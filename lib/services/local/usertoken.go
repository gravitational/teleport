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

// GetUserTokens returns all user tokens.
func (s *IdentityService) GetUserTokens(ctx context.Context) ([]types.UserToken, error) {
	startKey := backend.ExactKey(userTokenPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tokens []types.UserToken
	for _, item := range result.Items {
		if !item.Key.HasSuffix(backend.NewKey(paramsPrefix)) {
			continue
		}

		token, err := services.UnmarshalUserToken(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// DeleteUserToken deletes user token by ID.
func (s *IdentityService) DeleteUserToken(ctx context.Context, tokenID string) error {
	_, err := s.GetUserToken(ctx, tokenID)
	if err != nil {
		return trace.Wrap(err)
	}

	startKey := backend.ExactKey(userTokenPrefix, tokenID)
	if err = s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetUserToken returns a token by its ID.
func (s *IdentityService) GetUserToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	item, err := s.Get(ctx, backend.NewKey(userTokenPrefix, tokenID, paramsPrefix))
	switch {
	case trace.IsNotFound(err):
		return nil, trace.NotFound("user token(%s) not found", backend.MaskKeyName(tokenID))
	case err != nil:
		return nil, trace.Wrap(err)
	}

	token, err := services.UnmarshalUserToken(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// CreateUserToken creates a user token.
func (s *IdentityService) CreateUserToken(ctx context.Context, token types.UserToken) (types.UserToken, error) {
	if err := services.CheckAndSetDefaults(token); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalUserToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(userTokenPrefix, token.GetName(), paramsPrefix),
		Value:   value,
		Expires: token.Expiry(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// GetUserTokenSecrets returns token secrets.
func (s *IdentityService) GetUserTokenSecrets(ctx context.Context, tokenID string) (types.UserTokenSecrets, error) {
	item, err := s.Get(ctx, backend.NewKey(userTokenPrefix, tokenID, secretsPrefix))
	switch {
	case trace.IsNotFound(err):
		return nil, trace.NotFound("user token(%s) secrets not found", backend.MaskKeyName(tokenID))
	case err != nil:
		return nil, trace.Wrap(err)
	}

	secrets, err := services.UnmarshalUserTokenSecrets(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets, nil
}

// UpsertUserTokenSecrets upserts token secrets
func (s *IdentityService) UpsertUserTokenSecrets(ctx context.Context, secrets types.UserTokenSecrets) error {
	if err := services.CheckAndSetDefaults(secrets); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalUserTokenSecrets(secrets)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(userTokenPrefix, secrets.GetName(), secretsPrefix),
		Value:   value,
		Expires: secrets.Expiry(),
	}
	_, err = s.Put(ctx, item)

	return trace.Wrap(err)
}

const (
	userTokenPrefix = "usertoken"
	secretsPrefix   = "secrets"
)
