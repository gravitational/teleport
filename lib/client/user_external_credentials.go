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

package client

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/lib/client/credentialencryption"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const userExternalCredentialsCacheLockTimeout = 10 * time.Second

// SaveUserExternalCredentialsCache saves a local cache of the session's user external
// credentials.
func SaveUserExternalCredentialsCache(ctx context.Context, path string, resource *usercredpb.UserExternalCredentials) error {
	data, err := services.MarshalProtoResource(stripUserExternalCredentialsForCache(resource), services.PreserveRevision())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	unlock, err := utils.FSTryWriteLockTimeout(ctx, userExternalCredentialsCacheLockPath(path), userExternalCredentialsCacheLockTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()
	return trace.ConvertSystemError(os.WriteFile(path, data, 0o600))
}

// LoadUserExternalCredentialsCache loads the local cache of the session's user
// external credentials.
func LoadUserExternalCredentialsCache(ctx context.Context, path, encryptionKeyID string) (*usercredpb.UserExternalCredentials, error) {
	// Skip the lock when there is no cache.
	if _, err := os.Stat(path); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	unlock, err := utils.FSTryReadLockTimeout(ctx, userExternalCredentialsCacheLockPath(path), userExternalCredentialsCacheLockTimeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	resource, err := services.UnmarshalProtoResource[*usercredpb.UserExternalCredentials](data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// The cache may be left from a previous session.
	if resource.GetSpec().GetEncryptionKeyId().GetKeyId() != encryptionKeyID {
		return nil, trace.NotFound("user external credentials cache is not for the current session")
	}
	return resource, nil
}

func stripUserExternalCredentialsForCache(resource *usercredpb.UserExternalCredentials) *usercredpb.UserExternalCredentials {
	stripped := proto.CloneOf(resource)
	for _, credential := range stripped.GetSpec().GetCredentials() {
		// Drop OAuth refresh token
		if oauth := credential.GetOauth(); oauth != nil {
			oauth.SetRefreshTokenBlob(nil)
		}
	}
	return stripped
}

func userExternalCredentialsCacheLockPath(path string) string {
	return path + ".lock"
}

// deleteUserExternalCredentialsCache removes the local cache of the session's user
// external credentials, if any.
func deleteUserExternalCredentialsCache(ctx context.Context, path string) error {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	unlock, err := utils.FSTryWriteLockTimeout(ctx, userExternalCredentialsCacheLockPath(path), userExternalCredentialsCacheLockTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()
	if err := utils.RemoveSecure(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// ReencryptUserExternalCredentials returns the credentials (e.g. GitHub tokens)
// of ownCreds, decrypted with ownKey and encrypted with targetKey. It is
// expected to be called when ownCreds has credentials.
func ReencryptUserExternalCredentials(ctx context.Context, ownCreds *usercredpb.UserExternalCredentials, ownKey *ecdsa.PrivateKey, targetKey *ecdsa.PublicKey) ([]*usercredpb.Credential, error) {
	credentials := make([]*usercredpb.Credential, 0, len(ownCreds.GetSpec().GetCredentials()))
	for _, credential := range ownCreds.GetSpec().GetCredentials() {
		switch {
		case credential.GetOauth() != nil:
			reencrypted, err := reencryptOAuthCredential(credential, ownKey, targetKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			credentials = append(credentials, reencrypted)
		default:
			// Should not happen.
			log.DebugContext(ctx, "Skipping unsupported user external credential",
				"resource_kind", credential.GetResourceKind(),
				"resource_name", credential.GetResourceName(),
			)
		}
	}

	return credentials, nil
}

func reencryptOAuthCredential(credential *usercredpb.Credential, ownKey *ecdsa.PrivateKey, targetKey *ecdsa.PublicKey) (*usercredpb.Credential, error) {
	reencrypt := func(ciphertext []byte) ([]byte, error) {
		plaintext, err := credentialencryption.Decrypt(ownKey, ciphertext)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return credentialencryption.Encrypt(targetKey, plaintext)
	}

	oauth := credential.GetOauth()
	tokens := usercredpb.OAuthTokens_builder{
		AccessTokenExpiry:  oauth.GetAccessTokenExpiry(),
		RefreshTokenExpiry: oauth.GetRefreshTokenExpiry(),
	}
	var err error
	if tokens.AccessTokenBlob, err = reencrypt(oauth.GetAccessTokenBlob()); err != nil {
		return nil, trace.Wrap(err, "re-encrypting access token of %s %q", credential.GetResourceKind(), credential.GetResourceName())
	}
	if len(oauth.GetRefreshTokenBlob()) > 0 {
		if tokens.RefreshTokenBlob, err = reencrypt(oauth.GetRefreshTokenBlob()); err != nil {
			return nil, trace.Wrap(err, "re-encrypting refresh token of %s %q", credential.GetResourceKind(), credential.GetResourceName())
		}
	}
	return usercredpb.Credential_builder{
		ResourceKind: credential.GetResourceKind(),
		ResourceName: credential.GetResourceName(),
		Oauth:        tokens.Build(),
	}.Build(), nil
}
