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

package userexternalsecretsv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalsecrets/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptoutils"
	"github.com/gravitational/teleport/lib/services"
)

// SecretDecryptor decrypts auth-encrypted secret blobs.
type SecretDecryptor interface {
	DecryptTokens(ctx context.Context, ciphertext []byte) (accessToken, refreshToken string, err error)
}

// ServiceConfig holds configuration for the UserExternalSecretService.
type ServiceConfig struct {
	Authorizer         authz.Authorizer
	SessionCredentials services.UserSessionCredentials
	SecretDecryptor    SecretDecryptor
	Logger             *slog.Logger
}

// Service implements the UserExternalSecretService gRPC service.
type Service struct {
	pb.UnimplementedUserExternalSecretServiceServer

	authorizer         authz.Authorizer
	sessionCredentials services.UserSessionCredentials
	tokenDecryptor     SecretDecryptor
	logger             *slog.Logger
}

// NewService creates a new UserExternalSecretService.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.SessionCredentials == nil {
		return nil, trace.BadParameter("session_credentials is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Service{
		authorizer:         cfg.Authorizer,
		sessionCredentials: cfg.SessionCredentials,
		tokenDecryptor:     cfg.SecretDecryptor,
		logger:             cfg.Logger,
	}, nil
}

func getCallerEncryptionKeyID(authCtx *authz.Context) string {
	return authCtx.Identity.GetIdentity().EncryptionKeyID
}

// GetUserSessionCredentials returns the session credentials for the calling user.
func (s *Service) GetUserSessionCredentials(ctx context.Context, req *pb.GetUserSessionCredentialsRequest) (*pb.GetUserSessionCredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	encryptionKeyID := req.GetEncryptionKeyId()
	if encryptionKeyID == "" {
		encryptionKeyID = getCallerEncryptionKeyID(authCtx)
	}
	if encryptionKeyID == "" {
		return nil, trace.BadParameter("encryption key ID not provided and not found in caller's certificate")
	}

	username := authCtx.User.GetName()
	resource, err := s.sessionCredentials.GetByKeyID(ctx, username, encryptionKeyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pb.GetUserSessionCredentialsResponse_builder{
		Credentials: resource,
	}.Build(), nil
}

// DeleteUserSessionCredentials deletes the calling user's session credentials.
func (s *Service) DeleteUserSessionCredentials(ctx context.Context, req *pb.DeleteUserSessionCredentialsRequest) (*pb.DeleteUserSessionCredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	encryptionKeyID := getCallerEncryptionKeyID(authCtx)
	if encryptionKeyID == "" {
		return nil, trace.BadParameter("caller's certificate does not contain an encryption key ID")
	}

	username := authCtx.User.GetName()
	if err := s.sessionCredentials.Delete(ctx, username, encryptionKeyID); err != nil {
		if trace.IsNotFound(err) {
			return pb.DeleteUserSessionCredentialsResponse_builder{}.Build(), nil
		}
		return nil, trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Deleted session credentials",
		"user", username,
		"encryption_key_id", encryptionKeyID,
	)

	return pb.DeleteUserSessionCredentialsResponse_builder{}.Build(), nil
}

// UpdateUserSessionCredentials performs a conditional update on session credentials.
func (s *Service) UpdateUserSessionCredentials(ctx context.Context, req *pb.UpdateUserSessionCredentialsRequest) (*pb.UpdateUserSessionCredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetCredentials()
	if resource == nil {
		return nil, trace.BadParameter("credentials is required")
	}

	username := authCtx.User.GetName()
	if resource.GetSpec().GetUser() != username {
		return nil, trace.AccessDenied("cannot update credentials for another user")
	}

	if len(resource.GetSpec().GetCredentials()) > services.DefaultMaxSecretsPerSession {
		return nil, trace.LimitExceeded("session has too many credentials (max %d)", services.DefaultMaxSecretsPerSession)
	}

	updated, err := s.sessionCredentials.Update(ctx, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pb.UpdateUserSessionCredentialsResponse_builder{
		Credentials: updated,
	}.Build(), nil
}

// DecryptUserExternalSecret decrypts an auth-encrypted secret blob.
func (s *Service) DecryptUserExternalSecret(ctx context.Context, req *pb.DecryptUserExternalSecretRequest) (*pb.DecryptUserExternalSecretResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("DecryptUserExternalSecret is only available to proxy services")
	}

	authEncryptedBlob := req.GetAuthEncryptedBlob()
	if len(authEncryptedBlob) == 0 {
		return nil, trace.BadParameter("auth_encrypted_blob is required")
	}

	if s.tokenDecryptor == nil {
		return nil, trace.NotImplemented("token decryption is not configured")
	}

	payloadJSON, _, err := s.tokenDecryptor.DecryptTokens(ctx, authEncryptedBlob)
	if err != nil {
		return nil, trace.Wrap(err, "decrypting auth-encrypted blob")
	}

	payload, err := cryptoutils.UnmarshalEncryptedPayload([]byte(payloadJSON))
	if err != nil {
		return nil, trace.Wrap(err, "unmarshaling encrypted payload")
	}

	username := authCtx.Identity.GetIdentity().Username
	if payload.User != username {
		return nil, trace.AccessDenied("secret does not belong to this user")
	}

	return pb.DecryptUserExternalSecretResponse_builder{
		Plaintext: payload.Payload,
	}.Build(), nil
}
