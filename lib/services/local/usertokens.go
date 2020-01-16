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

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// GetUserTokens returns all UserTokens
func (s *IdentityService) GetUserTokens() ([]services.UserToken, error) {
	startKey := backend.Key(userTokensPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var usertokens []services.UserToken
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			continue
		}

		usertoken, err := services.UnmarshalUserToken(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		usertokens = append(usertokens, usertoken)
	}

	return usertokens, nil
}

// DeleteUserToken deletes UserToken by ID
func (s *IdentityService) DeleteUserToken(tokenID string) error {
	_, err := s.GetUserToken(tokenID)
	if err != nil {
		return trace.Wrap(err)
	}

	startKey := backend.Key(userTokensPrefix, tokenID)
	err = s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// GetUserToken returns a token by its ID
func (s *IdentityService) GetUserToken(tokenID string) (services.UserToken, error) {
	item, err := s.Get(context.TODO(), backend.Key(userTokensPrefix, tokenID, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user token(%v) not found", tokenID)
		}
		return nil, trace.Wrap(err)
	}

	usertoken, err := services.UnmarshalUserToken(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return usertoken, nil
}

// CreateUserToken creates a token that is used for signups and resets
func (s *IdentityService) CreateUserToken(usertoken services.UserToken) (services.UserToken, error) {
	if err := usertoken.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalUserToken(usertoken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(userTokensPrefix, usertoken.GetName(), paramsPrefix),
		Value:   value,
		Expires: usertoken.Expiry(),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return usertoken, nil
}

// GetUserTokenSecrets returns user token secrets
func (s *IdentityService) GetUserTokenSecrets(tokenID string) (services.UserTokenSecrets, error) {
	item, err := s.Get(context.TODO(), backend.Key(userTokensPrefix, tokenID, secretsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user token(%v) secrets not found", tokenID)
		}
		return nil, trace.Wrap(err)
	}

	secrets, err := services.UnmarshalUserTokenSecrets(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets, nil
}

// UpsertUserTokenSecrets upserts user token secrets
func (s *IdentityService) UpsertUserTokenSecrets(secrets services.UserTokenSecrets) error {
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalUserTokenSecrets(secrets)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(userTokensPrefix, secrets.GetName(), secretsPrefix),
		Value:   value,
		Expires: secrets.Expiry(),
	}
	_, err = s.Put(context.TODO(), item)

	return trace.Wrap(err)
}

const (
	userTokensPrefix = "usertokens"
	secretsPrefix    = "secrets"
)
