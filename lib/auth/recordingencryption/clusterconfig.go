// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package recordingencryption

import (
	"context"

	"github.com/gravitational/trace"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// ClusterConfigService wraps a services.ClusterConfigurationInternal and resolves recording encryption state whenever
// the session recording config is modified and indicates encryption is enabled.
type ClusterConfigService struct {
	services.ClusterConfigurationInternal
	resolver Resolver
}

// NewClusterConfigService returns a new ClusterConfigService.
func NewClusterConfigService(service services.ClusterConfigurationInternal, resolver Resolver) *ClusterConfigService {
	return &ClusterConfigService{
		ClusterConfigurationInternal: service,
		resolver:                     resolver,
	}
}

// CreateSessionRecordingConfig evaluates RecordingEncryption state before creating the SessionRecordingConfig.
func (s *ClusterConfigService) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	if !cfg.GetEncrypted() {
		res, err := s.ClusterConfigurationInternal.CreateSessionRecordingConfig(ctx, cfg)
		return res, trace.Wrap(err)
	}

	var res types.SessionRecordingConfig
	_, err := s.resolver.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
		var err error
		res, err = s.ClusterConfigurationInternal.CreateSessionRecordingConfig(ctx, cfg)
		return err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// UpdateSessionRecordingConfig evaluates RecordingEncryption state before updating the SessionRecordingConfig.
func (s *ClusterConfigService) UpdateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	if !cfg.GetEncrypted() {
		res, err := s.ClusterConfigurationInternal.UpdateSessionRecordingConfig(ctx, cfg)
		return res, trace.Wrap(err)
	}

	var res types.SessionRecordingConfig
	_, err := s.resolver.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
		var err error
		res, err = s.ClusterConfigurationInternal.UpdateSessionRecordingConfig(ctx, cfg)
		return err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// UpsertSessionRecordingConfig evaluates RecordingEncryption state before upserting the SessionRecordingConfig.
func (s *ClusterConfigService) UpsertSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	if !cfg.GetEncrypted() {
		res, err := s.ClusterConfigurationInternal.UpsertSessionRecordingConfig(ctx, cfg)
		return res, trace.Wrap(err)
	}

	var res types.SessionRecordingConfig
	_, err := s.resolver.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
		var err error
		res, err = s.ClusterConfigurationInternal.UpsertSessionRecordingConfig(ctx, cfg)
		return err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}
