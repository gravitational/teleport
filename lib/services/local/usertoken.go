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
	"bytes"
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// GetUserTokens returns all user tokens.
func (s *IdentityService) GetUserTokens(ctx context.Context) ([]types.UserToken, error) {
	startKey := backend.Key(userTokenPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN 9.0.0 retrieve tokens with old prefix.
	startKey = backend.Key(LegacyPasswordTokensPrefix)
	oldPrefixResult, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	items := append(result.Items, oldPrefixResult.Items...)

	var tokens []types.UserToken
	for _, item := range items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
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

	startKey := backend.Key(userTokenPrefix, backend.NewSafeString(tokenID))
	if err = s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}

	// DELETE IN 9.0.0 also delete any tokens with old prefix.
	startKey = backend.Key(LegacyPasswordTokensPrefix, backend.NewSafeString(tokenID))
	return trace.Wrap(s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// GetUserToken returns a token by its ID.
func (s *IdentityService) GetUserToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	item, err := s.Get(ctx, backend.Key(userTokenPrefix, backend.NewSafeString(tokenID), paramsPrefix))

	// DELETE IN 9.0.0: fallback for old prefix first.
	if trace.IsNotFound(err) {
		item, err = s.Get(ctx, backend.Key(LegacyPasswordTokensPrefix, backend.NewSafeString(tokenID), paramsPrefix))
	}

	// Handle errors from either Get.
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
	if err := token.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalUserToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(userTokenPrefix, backend.NewSafeString(token.GetName()), paramsPrefix),
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
	item, err := s.Get(ctx, backend.Key(userTokenPrefix, backend.NewSafeString(tokenID), secretsPrefix))

	// DELETE IN 9.0.0: fallback for old prefix first.
	if trace.IsNotFound(err) {
		item, err = s.Get(ctx, backend.Key(LegacyPasswordTokensPrefix, backend.NewSafeString(tokenID), secretsPrefix))
	}

	// Handle errors from either Get.
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
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalUserTokenSecrets(secrets)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(userTokenPrefix, backend.NewSafeString(secrets.GetName()), secretsPrefix),
		Value:   value,
		Expires: secrets.Expiry(),
	}
	_, err = s.Put(ctx, item)

	return trace.Wrap(err)
}

const (
	// DELETE IN 9.0.0 in favor of userTokenPrefix.
	LegacyPasswordTokensPrefix = "resetpasswordtokens"
	userTokenPrefix            = "usertoken"
	secretsPrefix              = "secrets"
)
