/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

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
	draftExternalAuditStorageBackendKey   = backend.NewKey(externalAuditStoragePrefix, externalAuditStorageDraftName)
	clusterExternalAuditStorageBackendKey = backend.NewKey(externalAuditStoragePrefix, externalAuditStorageClusterName)
)

// ExternalAuditStorageService manages External Audit Storage resources in the Backend.
type ExternalAuditStorageService struct {
	backend backend.Backend
	logger  *logrus.Entry
}

// NewExternalAuditStorageService returns a new *ExternalAuditStorageService or an error if it fails.
func NewExternalAuditStorageService(backend backend.Backend) *ExternalAuditStorageService {
	return &ExternalAuditStorageService{
		backend: backend,
		logger:  logrus.WithField(trace.Component, "ExternalAuditStorage.backend"),
	}
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
	item := backend.Item{
		Key:   draftExternalAuditStorageBackendKey,
		Value: value,
	}

	var lease *backend.Lease
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            s.backend,
			LockNameComponents: []string{externalAuditStorageLockName},
			TTL:                externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		// Check that the referenced AWS OIDC integration actually exists.
		if _, _, err := s.checkAWSIntegration(ctx, in.Spec.IntegrationName); err != nil {
			return trace.Wrap(err, "checking AWS OIDC integration")
		}

		lease, err = s.backend.Create(ctx, item)
		return trace.Wrap(err)
	})
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("draft external_audit_storage already exists")
		}
		return nil, trace.Wrap(err)
	}

	out := in.Clone()
	out.SetRevision(lease.Revision)
	return out, nil
}

// UpsertDraftExternalAudit upserts the draft External Audit Storage resource.
func (s *ExternalAuditStorageService) UpsertDraftExternalAuditStorage(ctx context.Context, in *externalauditstorage.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	value, err := services.MarshalExternalAuditStorage(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:   draftExternalAuditStorageBackendKey,
		Value: value,
	}

	var lease *backend.Lease
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            s.backend,
			LockNameComponents: []string{externalAuditStorageLockName},
			TTL:                externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		// Check that the referenced AWS OIDC integration actually exists.
		if _, _, err := s.checkAWSIntegration(ctx, in.Spec.IntegrationName); err != nil {
			return trace.Wrap(err, "checking AWS OIDC integration")
		}

		lease, err = s.backend.Put(ctx, item)
		return trace.Wrap(err)
	})
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("draft external_audit_storage already exists")
		}
		return nil, trace.Wrap(err)
	}

	out := in.Clone()
	out.SetRevision(lease.Revision)
	return out, nil
}

// GenerateDraftExternalAuditStorage creates a new draft ExternalAuditStorage with
// randomized resource names and stores it as the current draft, returning the
// generated resource.
func (s *ExternalAuditStorageService) GenerateDraftExternalAuditStorage(ctx context.Context, integrationName, region string) (*externalauditstorage.ExternalAuditStorage, error) {
	generated, err := externalauditstorage.GenerateDraftExternalAuditStorage(integrationName, region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.CreateDraftExternalAuditStorage(ctx, generated)
}

// DeleteDraftExternalAudit removes the draft ExternalAuditStorage resource.
func (s *ExternalAuditStorageService) DeleteDraftExternalAuditStorage(ctx context.Context) error {
	err := s.backend.Delete(ctx, draftExternalAuditStorageBackendKey)
	if trace.IsNotFound(err) {
		return trace.NotFound("draft external_audit_storage is not found")
	}
	return trace.Wrap(err)
}

// GetClusterExternalAuditStorage returns the cluster ExternalAuditStorage resource.
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

// PromoteToClusterExternalAuditStorage promotes the current draft to be the cluster ExternalAuditStorage
// resource.
func (s *ExternalAuditStorageService) PromoteToClusterExternalAuditStorage(ctx context.Context) error {
	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            s.backend,
			LockNameComponents: []string{externalAuditStorageLockName},
			TTL:                externalAuditStorageLockTTL,
		},
	}, func(ctx context.Context) error {
		draft, err := s.GetDraftExternalAuditStorage(ctx)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.BadParameter("can't promote to cluster when draft does not exist")
			}
			return trace.Wrap(err)
		}

		// Check that the referenced AWS OIDC integration actually exists.
		if _, _, err := s.checkAWSIntegration(ctx, draft.Spec.IntegrationName); err != nil {
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

// checkAWSIntegration checks that [integrationName] names an AWS OIDC integration that currently exists, and
// returns the backend key and revision if the AWS OIDC integration.
func (s *ExternalAuditStorageService) checkAWSIntegration(ctx context.Context, integrationName string) (key backend.Key, revision string, err error) {
	integrationsSvc, err := NewIntegrationsService(s.backend)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	integration, err := integrationsSvc.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, "", trace.Wrap(err, "getting integration")
	}
	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil, "", trace.BadParameter("%q is not an AWS OIDC integration", integrationName)
	}
	return integrationsSvc.svc.MakeKey(integrationName), integration.GetRevision(), nil
}

func getExternalAuditStorage(ctx context.Context, bk backend.Backend, key backend.Key) (*externalauditstorage.ExternalAuditStorage, error) {
	item, err := bk.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := services.UnmarshalExternalAuditStorage(item.Value, services.WithRevision(item.Revision), services.WithResourceID(item.ID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}
