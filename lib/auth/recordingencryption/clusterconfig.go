package recordingencryption

import (
	"context"

	"github.com/gravitational/trace"

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
	if cfg.GetEncrypted() {
		encryption, err := s.resolver.ResolveRecordingEncryption(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
	}

	res, err := s.ClusterConfigurationInternal.CreateSessionRecordingConfig(ctx, cfg)
	return res, trace.Wrap(err)
}

// UpdateSessionRecordingConfig evaluates RecordingEncryption state before updating the SessionRecordingConfig.
func (r *ClusterConfigService) UpdateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	if cfg.GetEncrypted() {
		encryption, err := r.resolver.ResolveRecordingEncryption(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
	}

	res, err := r.ClusterConfigurationInternal.UpdateSessionRecordingConfig(ctx, cfg)
	return res, trace.Wrap(err)
}

// UpsertSessionRecordingConfig evaluates RecordingEncryption state before upserting the SessionRecordingConfig.
func (r *ClusterConfigService) UpsertSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	if cfg.GetEncrypted() {
		encryption, err := r.resolver.ResolveRecordingEncryption(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
	}

	res, err := r.ClusterConfigurationInternal.UpsertSessionRecordingConfig(ctx, cfg)
	return res, trace.Wrap(err)
}
