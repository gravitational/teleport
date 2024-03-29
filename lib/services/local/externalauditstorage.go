/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	externalAuditStoragePrefix      = "external_audit_storage"
	externalAuditStorageDraftName   = "draft"
	externalAuditStorageClusterName = "cluster"
	externalAuditStorageLockName    = "external_audit_storage_lock"
	externalAuditStorageLockTTL     = 10 * time.Second
)

var (
	draftExternalAuditStorageBackendKey   = backend.Key(externalAuditStoragePrefix, externalAuditStorageDraftName)
	clusterExternalAuditStorageBackendKey = backend.Key(externalAuditStoragePrefix, externalAuditStorageClusterName)
)

// ExternalAuditStorageService manages External Audit Storage resources in the Backend.
type ExternalAuditStorageService struct {
	backend backend.Backend
	logger  *logrus.Entry
}

func NewExternalAuditStorageService(backend backend.Backend) *ExternalAuditStorageService {
	return &ExternalAuditStorageService{
		backend: backend,
		logger:  logrus.WithField(teleport.ComponentKey, "ExternalAuditStorage.backend"),
	}
}

// GetDraftExternalAuditStorage returns the draft External Audit Storage resource.
func (s *ExternalAuditStorageService) GetDraftExternalAuditStorage(ctx context.Context) (*externalauditstorage.ExternalAuditStorage, error) {
	item, err := s.backend.Get(ctx, draftExternalAuditStorageBackendKey)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster external_audit_storage is not found")
		}
		return nil, trace.Wrap(err)
	}
	out, err := services.UnmarshalExternalAuditStorage(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// CreateDraftExternalAudit creates the draft External Audit Storage resource if
// one does not already exist.
func (s *ExternalAuditStorageService) CreateDraftExternalAuditStorage(ctx context.Context, in *externalauditstorage.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	value, err := services.MarshalExternalAuditStorage(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Lock is used here and in Promote to prevent the possibility of deleting a
	// newly created draft after the previous one was promoted.
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalAuditStorageLockName,
			TTL:      externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		_, err = s.backend.Create(ctx, backend.Item{
			Key:   draftExternalAuditStorageBackendKey,
			Value: value,
		})
		return trace.Wrap(err)
	})
	if trace.IsAlreadyExists(err) {
		return nil, trace.AlreadyExists("draft external_audit_storage already exists")
	}
	return in, trace.Wrap(err)
}

// UpsertDraftExternalAudit upserts the draft External Audit Storage resource.
func (s *ExternalAuditStorageService) UpsertDraftExternalAuditStorage(ctx context.Context, in *externalauditstorage.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	value, err := services.MarshalExternalAuditStorage(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Lock is used here and in Promote to prevent upserting in the middle
	// of a promotion and the possibility of deleting a newly upserted draft
	// after the previous one was promoted.
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalAuditStorageLockName,
			TTL:      externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		_, err = s.backend.Put(ctx, backend.Item{
			Key:   draftExternalAuditStorageBackendKey,
			Value: value,
		})
		return trace.Wrap(err)
	})
	return in, trace.Wrap(err)
}

// GenerateDraftExternalAuditStorage creates a new draft ExternalAuditStorage with
// randomized resource names and stores it as the current draft, returning the
// generated resource.
func (s *ExternalAuditStorageService) GenerateDraftExternalAuditStorage(ctx context.Context, integrationName, region string) (*externalauditstorage.ExternalAuditStorage, error) {
	generated, err := externalauditstorage.GenerateDraftExternalAuditStorage(integrationName, region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalExternalAuditStorage(generated)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalAuditStorageLockName,
			TTL:      externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		_, err = s.backend.Create(ctx, backend.Item{
			Key:   draftExternalAuditStorageBackendKey,
			Value: value,
		})
		return trace.Wrap(err)
	})
	if trace.IsAlreadyExists(err) {
		return nil, trace.AlreadyExists("draft external_audit_storage already exists")
	}
	return generated, trace.Wrap(err)
}

// DeleteDraftExternalAudit removes the draft External Audit Storage resource.
func (s *ExternalAuditStorageService) DeleteDraftExternalAuditStorage(ctx context.Context) error {
	err := s.backend.Delete(ctx, draftExternalAuditStorageBackendKey)
	if trace.IsNotFound(err) {
		return trace.NotFound("draft external_audit_storage is not found")
	}
	return trace.Wrap(err)
}

// GetClusterExternalAuditStorage returns the cluster External Audit Storage resource.
func (s *ExternalAuditStorageService) GetClusterExternalAuditStorage(ctx context.Context) (*externalauditstorage.ExternalAuditStorage, error) {
	item, err := s.backend.Get(ctx, clusterExternalAuditStorageBackendKey)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster external_audit_storage is not found")
		}
		return nil, trace.Wrap(err)
	}
	out, err := services.UnmarshalExternalAuditStorage(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// PromoteToClusterExternalAuditStorage promotes draft to cluster external
// cloud audit resource.
func (s *ExternalAuditStorageService) PromoteToClusterExternalAuditStorage(ctx context.Context) error {
	// Lock is used here and in Create/Upsert/GenerateDraft to prevent upserting
	// in the middle of a promotion and the possibility of deleting a newly
	// created draft after the previous one was promoted.
	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalAuditStorageLockName,
			TTL:      externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		draft, err := s.GetDraftExternalAuditStorage(ctx)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.BadParameter("can't promote to cluster when draft does not exist")
			}
			return trace.Wrap(err)
		}
		out, err := externalauditstorage.NewClusterExternalAuditStorage(header.Metadata{}, draft.Spec)
		if err != nil {
			return trace.Wrap(err)
		}
		value, err := services.MarshalExternalAuditStorage(out)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = s.backend.Put(ctx, backend.Item{
			Key:   clusterExternalAuditStorageBackendKey,
			Value: value,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Clean up the current draft which has now been promoted.
		// Failing to delete the current draft is not critical and the promotion
		// has already succeeded, so just log any failure and return nil.
		if err := s.backend.Delete(ctx, draftExternalAuditStorageBackendKey); err != nil {
			s.logger.Info("failed to delete current draft external_audit_storage after promoting to cluster")
		}
		return nil
	})
	return trace.Wrap(err)
}

func (s *ExternalAuditStorageService) DisableClusterExternalAuditStorage(ctx context.Context) error {
	err := s.backend.Delete(ctx, clusterExternalAuditStorageBackendKey)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
