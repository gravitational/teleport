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
	"log/slog"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalsecrets/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const userSessionCredentialsPrefix = "user_external_credentials"

// EncryptionKeyIDFromPubKey derives a session encryption key ID from the
// public key bytes as a UUID v5.
func EncryptionKeyIDFromPubKey(pubKeyDER []byte) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, pubKeyDER).String()
}

// UserSessionCredentialsService manages per-user per-session credentials.
type UserSessionCredentialsService struct {
	backend backend.Backend
}

// NewUserSessionCredentialsService returns a new UserSessionCredentialsService.
func NewUserSessionCredentialsService(b backend.Backend) *UserSessionCredentialsService {
	return &UserSessionCredentialsService{backend: b}
}

// backendKeyForUser returns the backend key for a tsh session.
// Path: user_session_credentials/<user>/user/<key_id>
func (s *UserSessionCredentialsService) backendKeyForUser(user, keyID string) backend.Key {
	return backend.NewKey(userSessionCredentialsPrefix, user, "user", keyID)
}

// backendKeyForDelegation returns the backend key for a delegation session.
// Path: user_session_credentials/<user>/delegation/<delegation_session_id>/<bot_instance_id>
func (s *UserSessionCredentialsService) backendKeyForDelegation(user, delegationSessionID, botInstanceID string) backend.Key {
	return backend.NewKey(userSessionCredentialsPrefix, user, "delegation", delegationSessionID, botInstanceID)
}

// backendKeyFromResource determines the backend key from a UserSessionCredentials resource.
func (s *UserSessionCredentialsService) backendKeyFromResource(resource *pb.UserSessionCredentials) backend.Key {
	user := resource.GetSpec().GetUser()
	encKey := resource.GetSpec().GetEncryptionKey()
	delegationSessionID := encKey.GetDelegationSessionId()

	if delegationSessionID != "" {
		return s.backendKeyForDelegation(user, delegationSessionID, encKey.GetBotInstanceId())
	}
	return s.backendKeyForUser(user, encKey.GetKeyId())
}

// Get returns a UserSessionCredentials by user and key ID (tsh session).
func (s *UserSessionCredentialsService) Get(ctx context.Context, user, keyID string) (*pb.UserSessionCredentials, error) {
	item, err := s.backend.Get(ctx, s.backendKeyForUser(user, keyID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return unmarshalSessionCredentials(item)
}

// GetByDelegation returns a UserSessionCredentials by delegation session and bot instance.
func (s *UserSessionCredentialsService) GetByDelegation(ctx context.Context, user, delegationSessionID, botInstanceID string) (*pb.UserSessionCredentials, error) {
	item, err := s.backend.Get(ctx, s.backendKeyForDelegation(user, delegationSessionID, botInstanceID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return unmarshalSessionCredentials(item)
}

// GetByKeyID returns a UserSessionCredentials by key ID, searching both
// user and delegation paths.
func (s *UserSessionCredentialsService) GetByKeyID(ctx context.Context, user, keyID string) (*pb.UserSessionCredentials, error) {
	resource, err := s.Get(ctx, user, keyID)
	if err == nil {
		return resource, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// Not a tsh session key; search delegation sessions.
	resources, err := s.ListByUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, r := range resources {
		if r.GetSpec().GetEncryptionKey().GetKeyId() == keyID {
			return r, nil
		}
	}
	return nil, trace.NotFound("session credentials with key %s not found for user %s", keyID, user)
}

// Create creates a new UserSessionCredentials resource.
func (s *UserSessionCredentialsService) Create(ctx context.Context, resource *pb.UserSessionCredentials) (*pb.UserSessionCredentials, error) {
	value, err := services.MarshalProtoResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   s.backendKeyFromResource(resource),
		Value: value,
	}
	if exp := resource.GetMetadata().GetExpires(); exp != nil && exp.IsValid() {
		item.Expires = exp.AsTime()
	}
	lease, err := s.backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource.GetMetadata().Revision = lease.Revision
	return resource, nil
}

// Put creates or updates a UserSessionCredentials resource.
func (s *UserSessionCredentialsService) Put(ctx context.Context, resource *pb.UserSessionCredentials) (*pb.UserSessionCredentials, error) {
	value, err := services.MarshalProtoResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   s.backendKeyFromResource(resource),
		Value: value,
	}
	if exp := resource.GetMetadata().GetExpires(); exp != nil && exp.IsValid() {
		item.Expires = exp.AsTime()
	}
	lease, err := s.backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource.GetMetadata().Revision = lease.Revision
	return resource, nil
}

// Update updates an existing resource with CAS.
func (s *UserSessionCredentialsService) Update(ctx context.Context, resource *pb.UserSessionCredentials) (*pb.UserSessionCredentials, error) {
	value, err := services.MarshalProtoResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      s.backendKeyFromResource(resource),
		Value:    value,
		Revision: resource.GetMetadata().GetRevision(),
	}
	if exp := resource.GetMetadata().GetExpires(); exp != nil && exp.IsValid() {
		item.Expires = exp.AsTime()
	}
	lease, err := s.backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource.GetMetadata().Revision = lease.Revision
	return resource, nil
}

// Delete removes a tsh session's credentials.
func (s *UserSessionCredentialsService) Delete(ctx context.Context, user, keyID string) error {
	return trace.Wrap(s.backend.Delete(ctx, s.backendKeyForUser(user, keyID)))
}

// DeleteByDelegation removes a delegation session's credentials.
func (s *UserSessionCredentialsService) DeleteByDelegation(ctx context.Context, user, delegationSessionID, botInstanceID string) error {
	return trace.Wrap(s.backend.Delete(ctx, s.backendKeyForDelegation(user, delegationSessionID, botInstanceID)))
}

// DeleteByDelegationSession removes all credentials for a delegation session
// (across all bot instances).
func (s *UserSessionCredentialsService) DeleteByDelegationSession(ctx context.Context, user, delegationSessionID string) error {
	startKey := backend.NewKey(userSessionCredentialsPrefix, user, "delegation", delegationSessionID)
	items := s.backend.Items(ctx, backend.ItemsParams{
		StartKey: startKey,
		EndKey:   backend.RangeEnd(startKey),
	})
	for item, err := range items {
		if err != nil {
			return trace.Wrap(err)
		}
		if err := s.backend.Delete(ctx, item.Key); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ListByUser returns all UserSessionCredentials for a given user.
func (s *UserSessionCredentialsService) ListByUser(ctx context.Context, user string) ([]*pb.UserSessionCredentials, error) {
	startKey := backend.NewKey(userSessionCredentialsPrefix, user)
	items := s.backend.Items(ctx, backend.ItemsParams{
		StartKey: startKey,
		EndKey:   backend.RangeEnd(startKey),
	})

	var resources []*pb.UserSessionCredentials
	for item, err := range items {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resource, err := unmarshalSessionCredentials(&item)
		if err != nil {
			slog.WarnContext(ctx, "Failed to unmarshal user session credentials", "key", item.Key.String(), "error", err)
			continue
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func unmarshalSessionCredentials(item *backend.Item) (*pb.UserSessionCredentials, error) {
	resource, err := services.UnmarshalProtoResource[*pb.UserSessionCredentials](item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource.GetMetadata().Revision = item.Revision
	return resource, nil
}

// Event parser for watcher.

func newUserSessionCredentialsParser(m map[string]string) (*userSessionCredentialsParser, error) {
	var filter types.UserSessionCredentialsFilter
	if err := filter.FromMap(m); err != nil {
		return nil, trace.Wrap(err)
	}
	return &userSessionCredentialsParser{
		baseParser: newBaseParser(backend.NewKey(userSessionCredentialsPrefix)),
		filter:     filter,
	}, nil
}

type userSessionCredentialsParser struct {
	baseParser
	filter types.UserSessionCredentialsFilter
}

func (p *userSessionCredentialsParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		components := event.Item.Key.Components()
		if len(components) < 2 {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindUserSessionCredentials,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: components[1],
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*pb.UserSessionCredentials](event.Item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !p.filter.Match(resource.GetSpec().GetUser()) {
			return nil, nil
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
