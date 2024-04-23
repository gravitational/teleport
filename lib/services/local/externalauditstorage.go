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

// NewExternalAuditStorageService returns a new *ExternalAuditStorageService or an error if it fails.
func NewExternalAuditStorageService(backend backend.Backend) *ExternalAuditStorageService {
	return &ExternalAuditStorageService{
		backend: backend,
		logger:  logrus.WithField(teleport.ComponentKey, "ExternalAuditStorage.backend"),
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

	// First check if a draft already exists for a nicer error message than the one returned by AtomicWrite.
	_, err = s.backend.Get(ctx, draftExternalAuditStorageBackendKey)
	if err == nil {
		return nil, trace.AlreadyExists("draft external_audit_storage already exists")
	}

	// Check that the referenced AWS OIDC integration actually exists.
	integrationKey, integrationRevision, err := s.checkAWSIntegration(ctx, in.Spec.IntegrationName)
	if err != nil {
		return nil, trace.Wrap(err, "checking AWS OIDC integration")
	}

	revision, err := s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			// Make sure the AWS OIDC integration checked above hasn't changed.
			Key:       integrationKey,
			Condition: backend.Revision(integrationRevision),
			Action:    backend.Nop(),
		},
		{
			// Create the new draft EAS integration if one doesn't already exist.
			Key:       item.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(item),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := in.Clone()
	out.SetRevision(revision)
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

	// Check that the referenced AWS OIDC integration actually exists.
	integrationKey, integrationRevision, err := s.checkAWSIntegration(ctx, in.Spec.IntegrationName)
	if err != nil {
		return nil, trace.Wrap(err, "checking AWS OIDC integration")
	}

	revision, err := s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			// Make sure the AWS OIDC integration checked above hasn't changed.
			Key:       integrationKey,
			Condition: backend.Revision(integrationRevision),
			Action:    backend.Nop(),
		},
		{
			// Upsert the new draft EAS integration.
			Key:       item.Key,
			Condition: backend.Whatever(),
			Action:    backend.Put(item),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := in.Clone()
	out.SetRevision(revision)
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
	draft, err := s.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("can't promote to cluster when draft does not exist")
		}
		return trace.Wrap(err)
	}

	cluster, err := externalauditstorage.NewClusterExternalAuditStorage(header.Metadata{}, draft.Spec)
	if err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalExternalAuditStorage(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   clusterExternalAuditStorageBackendKey,
		Value: value,
	}

	integrationKey, integrationRevision, err := s.checkAWSIntegration(ctx, draft.Spec.IntegrationName)
	if err != nil {
		return trace.Wrap(err, "checking AWS OIDC integration")
	}

	_, err = s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			// Make sure the AWS OIDC integration checked above hasn't changed.
			Key:       integrationKey,
			Condition: backend.Revision(integrationRevision),
			Action:    backend.Nop(),
		},
		{
			// Make sure the draft EAS integration copied above hasn't changed, and delete it after the
			// promotion.
			Key:       draftExternalAuditStorageBackendKey,
			Condition: backend.Revision(draft.GetRevision()),
			Action:    backend.Delete(),
		},
		{
			// Upsert the new cluster EAS integration.
			Key:       item.Key,
			Condition: backend.Whatever(),
			Action:    backend.Put(item),
		},
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
func (s *ExternalAuditStorageService) checkAWSIntegration(ctx context.Context, integrationName string) (key []byte, revision string, err error) {
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

func getExternalAuditStorage(ctx context.Context, bk backend.Backend, key []byte) (*externalauditstorage.ExternalAuditStorage, error) {
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
