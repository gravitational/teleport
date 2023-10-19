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
}

func NewExternalCloudAuditService(backend backend.Backend) *ExternalCloudAuditService {
	return &ExternalCloudAuditService{
		backend: backend,
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
	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalCloudAuditLockName,
			TTL:      externalCloudAuditLockTTL,
		},
	}, func(ctx context.Context) error {
		return trace.Wrap(s.upsertDraftExternalCloudAuditLocked(ctx, in))
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return in, nil
}

func (s *ExternalCloudAuditService) upsertDraftExternalCloudAuditLocked(ctx context.Context, in *externalcloudaudit.ExternalCloudAudit) error {
	value, err := services.MarshalExternalCloudAudit(in)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.backend.Put(ctx, backend.Item{
		Key:   draftExternalCloudAuditBackendKey,
		Value: value,
	})
	return trace.Wrap(err)
}

// GenerateDraftExternalCloudAudit creates a new draft ExternalCloudAudit with
// randomized resource names and stores it as the current draft, returning the
// generated resource.
// To make this idempotent, if a draft ExternalCloudAudit is already present, it
// is not changed and is returned.
func (s *ExternalCloudAuditService) GenerateDraftExternalCloudAudit(ctx context.Context, integrationName, region string) (*externalcloudaudit.ExternalCloudAudit, error) {
	var draft *externalcloudaudit.ExternalCloudAudit

	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.backend,
			LockName: externalCloudAuditLockName,
			TTL:      externalCloudAuditLockTTL,
		},
	}, func(ctx context.Context) error {
		current, err := s.GetDraftExternalCloudAudit(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err == nil {
			if current.Spec.IntegrationName != integrationName {
				return trace.BadParameter(
					"a draft ExternalCloudAudit already exists with integration_name %q not matching given integration_name %q",
					current.Spec.IntegrationName, integrationName)
			}
			if current.Spec.Region != region {
				return trace.BadParameter(
					"a draft ExternalCloudAudit already exists with region %q not matching given region %q",
					current.Spec.IntegrationName, integrationName)
			}
			draft = current
			return nil
		}

		draft, err = externalcloudaudit.GenerateDraftExternalCloudAudit(integrationName, region)
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(s.upsertDraftExternalCloudAuditLocked(ctx, draft))
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return draft, nil
}

// DeleteDraftExternalAudit removes the draft external cloud audit resource.
func (s *ExternalCloudAuditService) DeleteDraftExternalCloudAudit(ctx context.Context) error {
	err := s.backend.Delete(ctx, draftExternalCloudAuditBackendKey)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	// Lock is used to prevent race between getting draft configuration and
	// upserting it while promoting/cloning.
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
				return trace.BadParameter("can't promote to cluster when draft does not exists")
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
		return trace.Wrap(err)
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
