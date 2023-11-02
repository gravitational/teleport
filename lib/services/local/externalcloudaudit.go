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

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	externalCloudAuditPrefix      = "external_cloud_audit"
	externalCloudAuditDraftName   = "draft"
	externalCloudAuditClusterName = "cluster"
	externalCloudAuditLockName    = "external_cloud_audit_lock"
	externalCloudAuditLockTTL     = 10 * time.Second
)

var (
	draftExternalCloudAuditBackendKey   = backend.Key(externalCloudAuditPrefix, externalCloudAuditDraftName)
	clusterExternalCloudAuditBackendKey = backend.Key(externalCloudAuditPrefix, externalCloudAuditClusterName)
)

// ExternalCloudAuditService manages external cloud audit resources in the Backend.
type ExternalCloudAuditService struct {
	backend backend.Backend
	logger  *logrus.Entry
}

func NewExternalCloudAuditService(backend backend.Backend) *ExternalCloudAuditService {
	return &ExternalCloudAuditService{
		backend: backend,
		logger:  logrus.WithField(trace.Component, "externalcloudaudit.backend"),
	}
}

// GetDraftExternalCloudAudit returns the draft external cloud audit resource.
func (s *ExternalCloudAuditService) GetDraftExternalCloudAudit(ctx context.Context) (*externalcloudaudit.ExternalCloudAudit, error) {
	item, err := s.backend.Get(ctx, draftExternalCloudAuditBackendKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := services.UnmarshalExternalCloudAudit(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// UpsertDraftExternalAudit upserts the draft external cloud audit resource.
func (s *ExternalCloudAuditService) UpsertDraftExternalCloudAudit(ctx context.Context, in *externalcloudaudit.ExternalCloudAudit) (*externalcloudaudit.ExternalCloudAudit, error) {
	value, err := services.MarshalExternalCloudAudit(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Lock is used here and in Promote to prevent upserting in the middle
	// of a promotion and the possibility of deleting a newly upserted draft
	// after the previous one was promoted.
	err = backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalCloudAuditLockName,
			TTL:      externalCloudAuditLockTTL,
		},
	}, func(ctx context.Context) error {
		_, err = s.backend.Put(ctx, backend.Item{
			Key:   draftExternalCloudAuditBackendKey,
			Value: value,
		})
		return trace.Wrap(err)
	})
	return in, trace.Wrap(err)
}

// GenerateDraftExternalCloudAudit creates a new draft ExternalCloudAudit with
// randomized resource names and stores it as the current draft, returning the
// generated resource.
func (s *ExternalCloudAuditService) GenerateDraftExternalCloudAudit(ctx context.Context, integrationName, region string) (*externalcloudaudit.ExternalCloudAudit, error) {
	generated, err := externalcloudaudit.GenerateDraftExternalCloudAudit(integrationName, region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := services.MarshalExternalCloudAudit(generated)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = s.backend.Create(ctx, backend.Item{
		Key:   draftExternalCloudAuditBackendKey,
		Value: value,
	})
	if trace.IsAlreadyExists(err) {
		return nil, trace.AlreadyExists("draft external_cloud_audit already exists")
	}
	return generated, trace.Wrap(err)
}

// DeleteDraftExternalAudit removes the draft external cloud audit resource.
func (s *ExternalCloudAuditService) DeleteDraftExternalCloudAudit(ctx context.Context) error {
	err := s.backend.Delete(ctx, draftExternalCloudAuditBackendKey)
	if trace.IsNotFound(err) {
		return trace.NotFound("draft external_cloud_audit is not found")
	}
	return trace.Wrap(err)
}

// GetClusterExternalCloudAudit returns the cluster external cloud audit resource.
func (s *ExternalCloudAuditService) GetClusterExternalCloudAudit(ctx context.Context) (*externalcloudaudit.ExternalCloudAudit, error) {
	item, err := s.backend.Get(ctx, clusterExternalCloudAuditBackendKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := services.UnmarshalExternalCloudAudit(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// PromoteToClusterExternalCloudAudit promotes draft to cluster external
// cloud audit resource.
func (s *ExternalCloudAuditService) PromoteToClusterExternalCloudAudit(ctx context.Context) error {
	// Lock is used here and in UpsertDraft to prevent upserting in the middle
	// of a promotion and the possibility of deleting a newly upserted draft
	// after the previous one was promoted.
	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalCloudAuditLockName,
			TTL:      externalCloudAuditLockTTL,
		},
	}, func(ctx context.Context) error {
		draft, err := s.GetDraftExternalCloudAudit(ctx)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.BadParameter("can't promote to cluster when draft does not exist")
			}
			return trace.Wrap(err)
		}
		out, err := externalcloudaudit.NewClusterExternalCloudAudit(header.Metadata{}, draft.Spec)
		if err != nil {
			return trace.Wrap(err)
		}
		value, err := services.MarshalExternalCloudAudit(out)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = s.backend.Put(ctx, backend.Item{
			Key:   clusterExternalCloudAuditBackendKey,
			Value: value,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Clean up the current draft which has now been promoted.
		// Failing to delete the current draft is not critical and the promotion
		// has already succeeded, so just log any failure and return nil.
		if err := s.backend.Delete(ctx, draftExternalCloudAuditBackendKey); err != nil {
			s.logger.Info("failed to delete current draft external_cloud_audit after promoting to cluster")
		}
		return nil
	})
	return trace.Wrap(err)
}

func (s *ExternalCloudAuditService) DisableClusterExternalCloudAudit(ctx context.Context) error {
	err := s.backend.Delete(ctx, clusterExternalCloudAuditBackendKey)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
