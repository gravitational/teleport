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

package userexternalcredentials

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"slices"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/lib/auth/credentialencryption"
	userencrypt "github.com/gravitational/teleport/lib/client/credentialencryption"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, "user-external-credentials"))

// Encryptor encrypts and decrypts the Auth-layer of user external credentials,
// e.g. [credentialencryption.Manager].
type Encryptor interface {
	// EncryptPayload encrypts the credential of the payload, bound to the user
	// and the resource of the payload.
	EncryptPayload(ctx context.Context, payload *credentialencryption.Payload) ([]byte, error)
	// DecryptPayload decrypts an Auth-encrypted payload.
	DecryptPayload(ctx context.Context, ciphertext []byte) (*credentialencryption.Payload, error)
}

// MakeAuthEncryptedOAuthCredential returns a credential with the OAuth tokens
// of the user for the resource, encrypted with the Auth-layer only. It must not
// be stored before it is encrypted for each session by Distribute.
//
// Callers that require an expiry should check it themselves for a clearer,
// provider-specific error.
func MakeAuthEncryptedOAuthCredential(ctx context.Context, encryptor Encryptor, username, resourceKind, resourceName string, token *oauth2.Token) (*usercredpb.Credential, error) {
	if token.AccessToken == "" {
		return nil, trace.BadParameter("missing access token")
	}

	encrypt := func(credential string) ([]byte, error) {
		return encryptor.EncryptPayload(ctx, &credentialencryption.Payload{
			User:         username,
			ResourceKind: resourceKind,
			ResourceName: resourceName,
			Credential:   []byte(credential),
		})
	}

	accessTokenBlob, err := encrypt(token.AccessToken)
	if err != nil {
		return nil, trace.Wrap(err, "encrypting access token")
	}

	var refreshTokenBlob []byte
	if token.RefreshToken != "" {
		if refreshTokenBlob, err = encrypt(token.RefreshToken); err != nil {
			return nil, trace.Wrap(err, "encrypting refresh token")
		}
	}

	oauth := usercredpb.OAuthTokens_builder{
		AccessTokenBlob:  accessTokenBlob,
		RefreshTokenBlob: refreshTokenBlob,
		// TODO(greedy52) set RefreshTokenExpiry when needed. OAuth 2.0 does
		// not define the refresh token lifetime, and providers that return it
		// use different fields (e.g. "refresh_token_expires_in" by GitHub).
	}.Build()
	// A zero token.Expiry means the provider did not return one, not that the
	// token is already expired.
	if !token.Expiry.IsZero() {
		oauth.SetAccessTokenExpiry(timestamppb.New(token.Expiry))
	}

	return usercredpb.Credential_builder{
		ResourceKind: resourceKind,
		ResourceName: resourceName,
		Oauth:        oauth,
	}.Build(), nil
}

// Distribute encrypts the Auth-encrypted credential with the encryption key of
// each session of the user and adds it to each session. An existing credential
// for the same resource is replaced.
func Distribute(ctx context.Context, clt services.UserExternalCredentials, username string, authEncrypted *usercredpb.Credential) error {
	// TODO(greedy52) improve handling on large number of sessions. E.g. one
	// option is to only update caller's session and do the rest in a background
	// go-routine with a jitter.
	const maxSessions = 25
	sessions, nextPageToken, err := clt.List(ctx, maxSessions, "", usercredpb.EncryptionKeyID_builder{
		User: username,
	}.Build())
	if err != nil {
		return trace.Wrap(err, "listing user external credentials")
	}

	log := logger.With("user", username, "kind", authEncrypted.GetResourceKind(), "resource", authEncrypted.GetResourceName())
	var errs []error
	for _, session := range sessions {
		err := addToSession(ctx, clt, session, authEncrypted)
		switch {
		case trace.IsNotFound(err):
			// The session has expired or been deleted since it was listed.
			log.DebugContext(ctx, "Session not found, skipping", "key_id", session.GetSpec().GetEncryptionKeyId())
			continue
		case err != nil:
			// Keep going so one bad session doesn't block the rest.
			errs = append(errs, trace.Wrap(err, "adding user external credential to session %s", session.GetSpec().GetEncryptionKeyId().GetKeyId()))
			continue
		}
		log.DebugContext(ctx, "Distributed user external credential", "key_id", session.GetSpec().GetEncryptionKeyId())
	}
	if nextPageToken != "" {
		log.WarnContext(ctx, "Too many sessions, user external credential may not be distributed to all sessions",
			"distributed", len(sessions),
		)
	}
	return trace.NewAggregate(errs...)
}

func addToSession(ctx context.Context, clt services.UserExternalCredentials, session *usercredpb.UserExternalCredentials, authEncrypted *usercredpb.Credential) error {
	// The maximum number of attempts to update the session on revision
	// conflicts.
	const maxAttempts = 3
	for attempt := 1; ; attempt++ {
		credential, err := encryptWithUserPublicKey(session.GetSpec().GetEncryptionPublicKey(), authEncrypted)
		if err != nil {
			return trace.Wrap(err)
		}

		credentials := slices.DeleteFunc(slices.Clone(session.GetSpec().GetCredentials()), func(c *usercredpb.Credential) bool {
			return c.GetResourceKind() == credential.GetResourceKind() && c.GetResourceName() == credential.GetResourceName()
		})
		session.GetSpec().SetCredentials(append(credentials, credential))

		if _, err := clt.Update(ctx, session); !trace.IsCompareFailed(err) || attempt >= maxAttempts {
			return trace.Wrap(err)
		}

		// Someone else updated the session. This should be rare but fetch the
		// latest and try again.
		session, err = clt.Get(ctx, session.GetSpec().GetEncryptionKeyId())
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func encryptWithUserPublicKey(encryptionPublicKeyDER []byte, authEncrypted *usercredpb.Credential) (*usercredpb.Credential, error) {
	parsed, err := x509.ParsePKIXPublicKey(encryptionPublicKeyDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing session encryption public key")
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("expected ECDSA session encryption public key, got %T", parsed)
	}

	credential := proto.CloneOf(authEncrypted)
	switch {
	case credential.GetOauth() != nil:
		if err := encryptOAuthInPlace(publicKey, credential.GetOauth()); err != nil {
			return nil, trace.Wrap(err, "encrypting OAuth tokens with user public key")
		}
	default:
		// Only OAuth credentials are supported at the moment.
		return nil, trace.BadParameter("unsupported user external credential for %s %q (this is a bug)", credential.GetResourceKind(), credential.GetResourceName())
	}
	return credential, nil
}

func encryptOAuthInPlace(publicKey *ecdsa.PublicKey, oauth *usercredpb.OAuthTokens) error {
	accessTokenBlob, err := userencrypt.Encrypt(publicKey, oauth.GetAccessTokenBlob())
	if err != nil {
		return trace.Wrap(err)
	}
	oauth.SetAccessTokenBlob(accessTokenBlob)
	if len(oauth.GetRefreshTokenBlob()) > 0 {
		refreshTokenBlob, err := userencrypt.Encrypt(publicKey, oauth.GetRefreshTokenBlob())
		if err != nil {
			return trace.Wrap(err)
		}
		oauth.SetRefreshTokenBlob(refreshTokenBlob)
	}
	return nil
}
