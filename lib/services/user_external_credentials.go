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

	"github.com/google/uuid"

	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
)

// UserExternalCredentials manages user external credentials for encrypted
// credential storage.
type UserExternalCredentials interface {
	Create(ctx context.Context, resource *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error)
	Update(ctx context.Context, resource *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error)
	Get(ctx context.Context, keyID *userexternalcredentialsv1.EncryptionKeyID) (*userexternalcredentialsv1.UserExternalCredentials, error)
	Delete(ctx context.Context, keyID *userexternalcredentialsv1.EncryptionKeyID) error
	List(ctx context.Context, pageSize int, pageToken string, filter *userexternalcredentialsv1.EncryptionKeyID) ([]*userexternalcredentialsv1.UserExternalCredentials, string, error)
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
