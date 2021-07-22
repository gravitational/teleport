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

// GetResetPasswordTokens returns all ResetPasswordTokens
func (s *IdentityService) GetResetPasswordTokens(ctx context.Context) ([]types.ResetPasswordToken, error) {
	startKey := backend.Key(passwordTokensPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tokens []types.ResetPasswordToken
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			continue
		}

		token, err := services.UnmarshalResetPasswordToken(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// DeleteResetPasswordToken deletes ResetPasswordToken by ID
func (s *IdentityService) DeleteResetPasswordToken(ctx context.Context, tokenID string) error {
	_, err := s.GetResetPasswordToken(ctx, tokenID)
	if err != nil {
		return trace.Wrap(err)
	}

	startKey := backend.Key(passwordTokensPrefix, tokenID)
	err = s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// GetResetPasswordToken returns a token by its ID
func (s *IdentityService) GetResetPasswordToken(ctx context.Context, tokenID string) (types.ResetPasswordToken, error) {
	item, err := s.Get(ctx, backend.Key(passwordTokensPrefix, tokenID, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("reset password token(%v) not found", tokenID)
		}
		return nil, trace.Wrap(err)
	}

	token, err := services.UnmarshalResetPasswordToken(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// CreateResetPasswordToken creates a token that is used for signups and resets
func (s *IdentityService) CreateResetPasswordToken(ctx context.Context, token types.ResetPasswordToken) (types.ResetPasswordToken, error) {
	if err := token.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalResetPasswordToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(passwordTokensPrefix, token.GetName(), paramsPrefix),
		Value:   value,
		Expires: token.Expiry(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// GetResetPasswordTokenSecrets returns token secrets
func (s *IdentityService) GetResetPasswordTokenSecrets(ctx context.Context, tokenID string) (types.ResetPasswordTokenSecrets, error) {
	item, err := s.Get(ctx, backend.Key(passwordTokensPrefix, tokenID, secretsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("reset password token(%v) secrets not found", tokenID)
		}
		return nil, trace.Wrap(err)
	}

	secrets, err := services.UnmarshalResetPasswordTokenSecrets(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets, nil
}

// UpsertResetPasswordTokenSecrets upserts token secrets
func (s *IdentityService) UpsertResetPasswordTokenSecrets(ctx context.Context, secrets types.ResetPasswordTokenSecrets) error {
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalResetPasswordTokenSecrets(secrets)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(passwordTokensPrefix, secrets.GetName(), secretsPrefix),
		Value:   value,
		Expires: secrets.Expiry(),
	}
	_, err = s.Put(ctx, item)

	return trace.Wrap(err)
}

const (
	passwordTokensPrefix = "resetpasswordtokens"
	secretsPrefix        = "secrets"
)
