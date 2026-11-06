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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	userExternalCredentialsPrefix = "user_external_credentials"
	mcpOAuthCredentialsPrefix     = "mcp_oauth_credentials"
)

// UserExternalCredentialsService manages user external credentials.
type UserExternalCredentialsService struct {
	service             *generic.ServiceWrapper[*pb.UserExternalCredentials]
	mcpOAuthCredentials *generic.ScopeAwareServiceWrapper[*pb.MCPOAuthCredential]
}

// NewUserExternalCredentialsService returns a new UserExternalCredentialsService.
func NewUserExternalCredentialsService(b backend.Backend) (*UserExternalCredentialsService, error) {
	service, err := generic.NewServiceWrapper(generic.ServiceConfig[*pb.UserExternalCredentials]{
		Backend:       b,
		ResourceKind:  types.KindUserExternalCredentials,
		BackendPrefix: backend.NewKey(userExternalCredentialsPrefix),
		MarshalFunc:   services.MarshalProtoResource[*pb.UserExternalCredentials],
		UnmarshalFunc: services.UnmarshalProtoResource[*pb.UserExternalCredentials],
		ValidateFunc:  validateUserExternalCredentials,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	mcpOAuthCredentials, err := generic.NewScopeAwareServiceWrapper(generic.ScopeAwareServiceWrapperConfig[*pb.MCPOAuthCredential]{
		Backend:               b,
		ResourceKind:          types.KindMCPOAuthCredential,
		UnscopedBackendPrefix: backend.NewKey(mcpOAuthCredentialsPrefix),
		ScopedBackendPrefix:   backend.NewKey(scopedPrefix, mcpOAuthCredentialsPrefix),
		MarshalFunc:           services.MarshalProtoResource[*pb.MCPOAuthCredential],
		UnmarshalFunc:         services.UnmarshalProtoResource[*pb.MCPOAuthCredential],
		ValidateFunc:          validateMCPOAuthCredential,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &UserExternalCredentialsService{service: service, mcpOAuthCredentials: mcpOAuthCredentials}, nil
}

func validateUserExternalCredentials(resource *pb.UserExternalCredentials) error {
	if resource.GetKind() != types.KindUserExternalCredentials {
		return trace.BadParameter("invalid kind %q", resource.GetKind())
	}
	keyID := resource.GetSpec().GetEncryptionKeyId()
	if err := validateUserExternalCredentialsKeyID(keyID); err != nil {
		return trace.Wrap(err)
	}
	if err := checkEncryptionPublicKey(resource.GetSpec().GetEncryptionPublicKey()); err != nil {
		return trace.Wrap(err)
	}
	if resource.GetMetadata().GetExpires() == nil {
		return trace.BadParameter("expiry is required")
	}
	const maxCredentials = 10
	if len(resource.GetSpec().GetCredentials()) > maxCredentials {
		return trace.BadParameter("too many credentials, at most %d are allowed", maxCredentials)
	}
	for _, credential := range resource.GetSpec().GetCredentials() {
		if credential.GetResourceKind() == "" || credential.GetResourceName() == "" {
			return trace.BadParameter("credential is missing resource kind or name")
		}
		if len(credential.GetOauth().GetAccessTokenBlob()) == 0 {
			return trace.BadParameter("credential for %s %q has no access token", credential.GetResourceKind(), credential.GetResourceName())
		}
	}
	return nil
}

func checkEncryptionPublicKey(der []byte) error {
	if len(der) == 0 {
		return trace.BadParameter("missing encryption_public_key")
	}
	// A DER-encoded P-256 key is 91 bytes.
	if len(der) > 1024 {
		return trace.BadParameter("encryption_public_key is too large")
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return trace.BadParameter("parsing encryption_public_key: %v", err)
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return trace.BadParameter("expected ECDSA encryption_public_key, got %T", parsed)
	}
	if publicKey.Curve != elliptic.P256() {
		return trace.BadParameter("encryption_public_key must be P-256")
	}
	return nil
}

// backendKeyForUserExternalCredentials returns <user>/user/<key_id> for the
// user's own sessions, or <user>/delegation/<delegate>/<key_id> for sessions of
// a delegate of the user. Without key_id, it returns the prefix for listing.
func backendKeyForUserExternalCredentials(keyID *pb.EncryptionKeyID) backend.Key {
	components := []string{keyID.GetUser()}
	if keyID.GetDelegate() != "" {
		components = append(components, "delegation", keyID.GetDelegate())
		if keyID.GetKeyId() != "" {
			components = append(components, keyID.GetKeyId())
		}
	} else if keyID.GetKeyId() != "" {
		components = append(components, "user", keyID.GetKeyId())
	}
	return backend.NewKey(components...)
}

func validateUserExternalCredentialsKeyID(keyID *pb.EncryptionKeyID) error {
	if keyID == nil {
		return trace.BadParameter("missing encryption_key_id")
	}
	if keyID.GetUser() == "" {
		return trace.BadParameter("missing encryption_key_id.user")
	}
	if keyID.GetKeyId() == "" {
		return trace.BadParameter("missing encryption_key_id.key_id")
	}

	if !backend.IsKeySafe(backendKeyForUserExternalCredentials(keyID)) {
		return trace.BadParameter("encryption_key_id contains invalid characters")
	}
	return nil
}

func validateUserExternalCredentialsKeyIDForList(keyID *pb.EncryptionKeyID) error {
	if keyID == nil {
		return trace.BadParameter("missing encryption_key_id")
	}
	if keyID.GetUser() == "" {
		return trace.BadParameter("missing encryption_key_id.user")
	}

	if !backend.IsKeySafe(backendKeyForUserExternalCredentials(keyID)) {
		return trace.BadParameter("encryption_key_id contains invalid characters")
	}
	return nil
}

func (s *UserExternalCredentialsService) serviceForKeyID(keyID *pb.EncryptionKeyID) (*generic.ServiceWrapper[*pb.UserExternalCredentials], error) {
	if err := validateUserExternalCredentialsKeyID(keyID); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.service.WithNameKeyFunc(func() backend.Key {
		return backendKeyForUserExternalCredentials(keyID)
	}), nil
}

func (s *UserExternalCredentialsService) prepareNewResource(resource *pb.UserExternalCredentials) error {
	if resource == nil {
		return trace.BadParameter("missing user external credentials")
	}
	if resource.GetMetadata().GetName() == "" {
		if resource.GetMetadata() == nil {
			resource.SetMetadata(&headerpb.Metadata{})
		}
		resource.GetMetadata().SetName(uuid.NewString())
	}
	return nil
}

// CreateUserExternalCredentials creates a new UserExternalCredentials resource.
func (s *UserExternalCredentialsService) CreateUserExternalCredentials(ctx context.Context, resource *pb.UserExternalCredentials) (*pb.UserExternalCredentials, error) {
	if err := s.prepareNewResource(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	svc, err := s.serviceForKeyID(resource.GetSpec().GetEncryptionKeyId())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := svc.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// UpsertUserExternalCredentials creates or overwrites a UserExternalCredentials resource.
func (s *UserExternalCredentialsService) UpsertUserExternalCredentials(ctx context.Context, resource *pb.UserExternalCredentials) (*pb.UserExternalCredentials, error) {
	if err := s.prepareNewResource(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	svc, err := s.serviceForKeyID(resource.GetSpec().GetEncryptionKeyId())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	upserted, err := svc.UpsertResource(ctx, resource)
	return upserted, trace.Wrap(err)
}

// UpdateUserExternalCredentials conditionally updates a UserExternalCredentials resource.
func (s *UserExternalCredentialsService) UpdateUserExternalCredentials(ctx context.Context, resource *pb.UserExternalCredentials) (*pb.UserExternalCredentials, error) {
	svc, err := s.serviceForKeyID(resource.GetSpec().GetEncryptionKeyId())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := svc.ConditionalUpdateResource(ctx, resource)
	return updated, trace.Wrap(err)
}

// GetUserExternalCredentials returns a UserExternalCredentials by key ID.
func (s *UserExternalCredentialsService) GetUserExternalCredentials(ctx context.Context, keyID *pb.EncryptionKeyID) (*pb.UserExternalCredentials, error) {
	svc, err := s.serviceForKeyID(keyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource, err := svc.GetResource(ctx, "")
	return resource, trace.Wrap(err)
}

// DeleteUserExternalCredentials removes a UserExternalCredentials resource.
func (s *UserExternalCredentialsService) DeleteUserExternalCredentials(ctx context.Context, keyID *pb.EncryptionKeyID) error {
	svc, err := s.serviceForKeyID(keyID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(svc.DeleteResource(ctx, ""))
}

// ListUserExternalCredentials returns a page of UserExternalCredentials
// matching the filter. The user field is required. Other fields narrow
// results by key path prefix.
func (s *UserExternalCredentialsService) ListUserExternalCredentials(ctx context.Context, pageSize int, pageToken string, filter *pb.EncryptionKeyID) ([]*pb.UserExternalCredentials, string, error) {
	if err := validateUserExternalCredentialsKeyIDForList(filter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	prefix := backendKeyForUserExternalCredentials(filter)
	return s.service.WithPrefix(prefix.Components()...).ListResources(ctx, pageSize, pageToken)
}

func validateMCPOAuthCredential(resource *pb.MCPOAuthCredential) error {
	if resource.GetKind() != types.KindMCPOAuthCredential {
		return trace.BadParameter("invalid kind %q", resource.GetKind())
	}
	if resource.GetMetadata().GetName() == "" {
		return trace.BadParameter("missing app name")
	}
	if len(resource.GetSpec().GetEncrypted()) == 0 {
		return trace.BadParameter("missing encrypted credential")
	}
	return nil
}

// mcpOAuthCredentialService returns the service for a user's credentials for apps
// in the scope.
func (s *UserExternalCredentialsService) mcpOAuthCredentialService(user, scope string) (*generic.ServiceWrapper[*pb.MCPOAuthCredential], error) {
	service, err := s.mcpOAuthCredentials.WithScopePrefix(scope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return service.WithPrefix(user), nil
}

// GetMCPOAuthCredential returns a user's encrypted OAuth credential for an MCP
// server.
func (s *UserExternalCredentialsService) GetMCPOAuthCredential(ctx context.Context, user string, app apiscopes.QualifiedName) ([]byte, error) {
	service, err := s.mcpOAuthCredentialService(user, app.Scope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credential, err := service.GetResource(ctx, app.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return credential.GetSpec().GetEncrypted(), nil
}

// UpsertMCPOAuthCredential stores a user's encrypted OAuth credential for an
// MCP server.
func (s *UserExternalCredentialsService) UpsertMCPOAuthCredential(ctx context.Context, user string, app apiscopes.QualifiedName, encrypted []byte) error {
	service, err := s.mcpOAuthCredentialService(user, app.Scope)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = service.UpsertResource(ctx, pb.MCPOAuthCredential_builder{
		Kind:     types.KindMCPOAuthCredential,
		Version:  types.V1,
		Metadata: headerpb.Metadata_builder{Name: app.Name}.Build(),
		Spec:     pb.MCPOAuthCredentialSpec_builder{Encrypted: encrypted}.Build(),
		Scope:    app.Scope,
	}.Build())
	return trace.Wrap(err)
}

// RunWhileMCPOAuthCredentialLocked runs fn while holding a lock on a user's
// credential for an MCP server.
func (s *UserExternalCredentialsService) RunWhileMCPOAuthCredentialLocked(ctx context.Context, user string, app apiscopes.QualifiedName, fn func(context.Context) error) error {
	service, err := s.mcpOAuthCredentialService(user, app.Scope)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(service.RunWhileLocked(ctx, service.BackendKey(app.Name).Components(), time.Minute, func(ctx context.Context, _ backend.Backend) error {
		return fn(ctx)
	}))
}
