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

package services

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
)

// UserExternalCredentials manages user external credentials for encrypted
// credential storage.
type UserExternalCredentials interface {
	CreateUserExternalCredentials(ctx context.Context, resource *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error)
	UpsertUserExternalCredentials(ctx context.Context, resource *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error)
	UpdateUserExternalCredentials(ctx context.Context, resource *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error)
	GetUserExternalCredentials(ctx context.Context, keyID *userexternalcredentialsv1.EncryptionKeyID) (*userexternalcredentialsv1.UserExternalCredentials, error)
	DeleteUserExternalCredentials(ctx context.Context, keyID *userexternalcredentialsv1.EncryptionKeyID) error
	ListUserExternalCredentials(ctx context.Context, pageSize int, pageToken string, filter *userexternalcredentialsv1.EncryptionKeyID) ([]*userexternalcredentialsv1.UserExternalCredentials, string, error)
}

// MakeUserEncryptionKeyID derives a deterministic encryption key ID from the
// DER-encoded public key bytes. Returns empty string if the key is empty.
func MakeUserEncryptionKeyID(pubKeyDER []byte) string {
	if len(pubKeyDER) == 0 {
		return ""
	}
	// Unfortunately stdlib uuid does not support UUID v5 yet.
	return uuid.NewSHA1(uuid.NameSpaceOID, pubKeyDER).String()
}

// FindUserExternalCredential returns the credential for the given resource.
func FindUserExternalCredential(resource *userexternalcredentialsv1.UserExternalCredentials, resourceKind, resourceName string) (*userexternalcredentialsv1.Credential, bool) {
	for _, credential := range resource.GetSpec().GetCredentials() {
		if credential.GetResourceKind() == resourceKind && credential.GetResourceName() == resourceName {
			return credential, true
		}
	}
	return nil, false
}

// PutUserExternalCredential adds the credential to the resource. An existing
// credential for the same resource is replaced only if the access token of the
// new credential expires later.
func PutUserExternalCredential(resource *userexternalcredentialsv1.UserExternalCredentials, credential *userexternalcredentialsv1.Credential) {
	credentials := resource.GetSpec().GetCredentials()
	i := slices.IndexFunc(credentials, func(c *userexternalcredentialsv1.Credential) bool {
		return c.GetResourceKind() == credential.GetResourceKind() && c.GetResourceName() == credential.GetResourceName()
	})
	switch {
	case i < 0:
		credentials = append(credentials, credential)
	case credential.GetOauth().GetAccessTokenExpiry().AsTime().After(credentials[i].GetOauth().GetAccessTokenExpiry().AsTime()):
		credentials[i] = credential
	}
	resource.GetSpec().SetCredentials(credentials)
}

// MakeDelegateEncryptionKeyID returns the composite key ID for a delegate's
// session encryption key.
func MakeDelegateEncryptionKeyID(delegate UserState, encryptionPublicKey []byte) (*userexternalcredentialsv1.EncryptionKeyID, error) {
	chain := delegate.GetDelegation().GetChain()
	if len(chain) == 0 {
		return nil, trace.BadParameter("cannot find delegator for bot %s", delegate.GetName())
	}
	rootDelegator, err := DelegatorUsername(chain[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userexternalcredentialsv1.EncryptionKeyID_builder{
		User:     rootDelegator,
		KeyId:    MakeUserEncryptionKeyID(encryptionPublicKey),
		Delegate: delegate.GetName(),
	}.Build(), nil
}
