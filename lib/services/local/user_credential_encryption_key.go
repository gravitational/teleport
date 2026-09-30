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

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const credentialEncryptionKeyPrefix = "user_credential_encryption_key"

// CredentialEncryptionKeyService manages the cluster's credential encryption
// key singleton.
type CredentialEncryptionKeyService struct {
	service *generic.ServiceWrapper[*pb.CredentialEncryptionKey]
}

// NewCredentialEncryptionKeyService returns a new CredentialEncryptionKeyService.
func NewCredentialEncryptionKeyService(b backend.Backend) (*CredentialEncryptionKeyService, error) {
	service, err := generic.NewServiceWrapper(generic.ServiceConfig[*pb.CredentialEncryptionKey]{
		Backend:       b,
		ResourceKind:  types.KindCredentialEncryptionKey,
		BackendPrefix: backend.NewKey(credentialEncryptionKeyPrefix),
		MarshalFunc:   services.MarshalProtoResource[*pb.CredentialEncryptionKey],
		UnmarshalFunc: services.UnmarshalProtoResource[*pb.CredentialEncryptionKey],
		ValidateFunc:  validateCredentialEncryptionKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CredentialEncryptionKeyService{service: service}, nil
}

func validateCredentialEncryptionKey(resource *pb.CredentialEncryptionKey) error {
	if resource.GetKind() != types.KindCredentialEncryptionKey {
		return trace.BadParameter("invalid kind %q", resource.GetKind())
	}
	if resource.GetMetadata().GetName() != types.MetaNameCredentialEncryptionKey {
		return trace.BadParameter("invalid name %q, expected %q", resource.GetMetadata().GetName(), types.MetaNameCredentialEncryptionKey)
	}
	if len(resource.GetSpec().GetPrivateKey()) == 0 {
		return trace.BadParameter("missing private_key")
	}
	if _, ok := types.PrivateKeyType_value[resource.GetSpec().GetPrivateKeyType()]; !ok {
		return trace.BadParameter("invalid private_key_type %q", resource.GetSpec().GetPrivateKeyType())
	}
	if resource.GetMetadata().GetExpires() != nil {
		return trace.BadParameter("credential encryption key must not have an expiry")
	}
	return nil
}

// Get returns the credential encryption key.
func (s *CredentialEncryptionKeyService) Get(ctx context.Context) (*pb.CredentialEncryptionKey, error) {
	resource, err := s.service.GetResource(ctx, types.MetaNameCredentialEncryptionKey)
	return resource, trace.Wrap(err)
}

// Create creates the credential encryption key. Returns AlreadyExists if the
// key already exists.
func (s *CredentialEncryptionKeyService) Create(ctx context.Context, resource *pb.CredentialEncryptionKey) (*pb.CredentialEncryptionKey, error) {
	created, err := s.service.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// Upsert creates or replaces the credential encryption key.
func (s *CredentialEncryptionKeyService) Upsert(ctx context.Context, resource *pb.CredentialEncryptionKey) (*pb.CredentialEncryptionKey, error) {
	upserted, err := s.service.UpsertResource(ctx, resource)
	return upserted, trace.Wrap(err)
}

// Delete removes the credential encryption key.
func (s *CredentialEncryptionKeyService) Delete(ctx context.Context) error {
	return trace.Wrap(s.service.DeleteResource(ctx, types.MetaNameCredentialEncryptionKey))
}
