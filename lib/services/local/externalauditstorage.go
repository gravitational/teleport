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
	backend         backend.Backend
	integrationsSvc *IntegrationsService
	// TODO(nklaassen): delete this once teleport.e is updated
	skipOIDCIntegrationCheck bool
	logger                   *logrus.Entry
}

// NewExternalAuditStorageServiceFallible returns a new *ExternalAuditStorageService or an error if it fails.
// TODO(nklaassen): once teleport.e is updated unify this with NewExternalAuditStorage.
func NewExternalAuditStorageServiceFallible(backend backend.Backend) (*ExternalAuditStorageService, error) {
	integrationsSvc, err := NewIntegrationsService(backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ExternalAuditStorageService{
		backend:         backend,
		integrationsSvc: integrationsSvc,
		logger:          logrus.WithField(teleport.ComponentKey, "ExternalAuditStorage.backend"),
	}, nil
}

// NewExternalAuditStorageServiceFallible returns a new *ExternalAuditStorageService.
// TODO(nklaassen): once teleport.e is updated unify this with NewExternalAuditStorageServiceFallible.
func NewExternalAuditStorageService(backend backend.Backend) *ExternalAuditStorageService {
	svc, err := NewExternalAuditStorageServiceFallible(backend)
	if err != nil {
		panic(err)
	}
	svc.skipOIDCIntegrationCheck = true
	return svc
}

// GetDraftExternalAuditStorage returns the draft External Audit Storage resource.
func (s *ExternalAuditStorageService) GetDraftExternalAuditStorage(ctx context.Context) (*externalauditstorage.ExternalAuditStorage, error) {
	eas, err := getExternalAuditStorage(ctx, s.backend, draftExternalAuditStorageBackendKey)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("draft external_audit_storage is not found")
		}
		return nil, trace.Wrap(err)
	}
	return eas, nil
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
		if err := s.checkOIDCIntegration(ctx, in.Spec.IntegrationName); err != nil {
			return trace.Wrap(err, "checking AWS OIDC integration")
		}
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
		if err := s.checkOIDCIntegration(ctx, in.Spec.IntegrationName); err != nil {
			return trace.Wrap(err, "checking AWS OIDC integration")
		}
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
		if err := s.checkOIDCIntegration(ctx, integrationName); err != nil {
			return trace.Wrap(err, "checking AWS OIDC integration")
		}
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
	eas, err := getExternalAuditStorage(ctx, s.backend, clusterExternalAuditStorageBackendKey)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster external_audit_storage is not found")
		}
		return nil, trace.Wrap(err)
	}
	return eas, nil
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
		if err := s.checkOIDCIntegration(ctx, draft.Spec.IntegrationName); err != nil {
			return trace.Wrap(err, "checking AWS OIDC integration")
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

// DisableClusterExternalAuditStorage disables External Audit Storage in the cluster by deleting the cluster
// EAS configuration.
func (s *ExternalAuditStorageService) DisableClusterExternalAuditStorage(ctx context.Context) error {
	err := s.backend.Delete(ctx, clusterExternalAuditStorageBackendKey)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *ExternalAuditStorageService) checkOIDCIntegration(ctx context.Context, integrationName string) error {
	// TODO(nklaassen): delete this once teleport.e is updated.
	if s.skipOIDCIntegrationCheck {
		return nil
	}
	integration, err := s.integrationsSvc.GetIntegration(ctx, integrationName)
	if err != nil {
		return trace.Wrap(err, "getting integration")
	}
	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return trace.BadParameter("%q is not an AWS OIDC integration", integrationName)
	}
	return nil
}

func getExternalAuditStorage(ctx context.Context, bk backend.Backend, key []byte) (*externalauditstorage.ExternalAuditStorage, error) {
	item, err := bk.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := services.UnmarshalExternalAuditStorage(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// EASIntegrationPreDeleteCheck should be called before deleting any AWS OIDC integration to be sure that it is
// not referenced by any EAS integration. If it returns any non-nil error, the integration should not be
// deleted. If it returns a nil error, [release] must be called after deleting the integration (or in any
// error case) to release a lock acquired by this check.
func EASIntegrationPreDeleteCheck(ctx context.Context, bk backend.Backend, integrationName string) (release func(), err error) {
	// It's necessary to do this check under the same lock that is used to create and promote EAS
	// configurations to avoid a race condition where an AWS OIDC configuration could be deleted immediately
	// after a new EAS configuration is added.
	lock, err := backend.AcquireLock(ctx, backend.LockConfiguration{
		Backend:  bk,
		LockName: externalAuditStorageLockName,
		TTL:      externalAuditStorageLockTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Be sure to release the lock if returning an error, otherwise the caller will be responsible for releasing it.
	defer func() {
		if err != nil {
			lock.Release(ctx, bk)
		}
	}()

	for _, key := range [][]byte{draftExternalAuditStorageBackendKey, clusterExternalAuditStorageBackendKey} {
		eas, err := getExternalAuditStorage(ctx, bk, key)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if eas.Spec.IntegrationName == integrationName {
			return nil, trace.BadParameter("cannot delete AWS OIDC integration currently referenced by External Audit Storage integration")
		}
	}

	release = func() {
		lock.Release(ctx, bk)
	}
	return release, nil
}
