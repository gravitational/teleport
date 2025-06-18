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
	"iter"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

// Resolver resolves RecordingEncryption state and passes the result to a postProcessFn callback to be called
// before any locks are released.
type Resolver interface {
	ResolveRecordingEncryption(ctx context.Context, postProcessFn func(context.Context, *recordingencryptionv1.RecordingEncryption) error) (*recordingencryptionv1.RecordingEncryption, error)
}

// WatchConfig captures required dependencies for building a RecordingEncryption watcher that
// automatically resolves state.
type WatchConfig struct {
	Events        types.Events
	Resolver      Resolver
	ClusterConfig services.ClusterConfiguration
	Logger        *slog.Logger
}

// A Watcher watches for changes to the RecordingEncryption resource and resolves the state for the calling
// auth server.
type Watcher struct {
	events        types.Events
	resolver      Resolver
	clusterConfig services.ClusterConfiguration
	logger        *slog.Logger
}

// NewWatcher returns a new Watcher.
func NewWatcher(cfg WatchConfig) (*Watcher, error) {
	switch {
	case cfg.Events == nil:
		return nil, trace.BadParameter("events is required")
	case cfg.Resolver == nil:
		return nil, trace.BadParameter("recording encryption resolver is required")
	case cfg.ClusterConfig == nil:
		return nil, trace.BadParameter("cluster config backend is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "encryption-watcher")
	}

	return &Watcher{
		events:        cfg.Events,
		resolver:      cfg.Resolver,
		clusterConfig: cfg.ClusterConfig,
		logger:        cfg.Logger,
	}, nil
}

// Watch creates a watcher responsible for responding to changes in the RecordingEncryption resource.
// This is how auth servers cooperate and ensure there are accessible wrapped keys for each unique keystore
// configuration in a cluster.
func (w *Watcher) Run(ctx context.Context) (err error) {
	// shouldRetryAfterJitterFn waits at most 5 seconds and returns a bool specifying whether or not
	// execution should continue
	shouldRetryAfterJitterFn := func() bool {
		select {
		case <-time.After(retryutils.SeventhJitter(time.Second * 5)):
			return true
		case <-ctx.Done():
			return false
		}
	}

	defer func() {
		w.logger.InfoContext(ctx, "stopping encryption watcher", "error", err)
	}()

	for {
		watch, err := w.events.NewWatcher(ctx, types.Watch{
			Name: "recording_encryption_watcher",
			Kinds: []types.WatchKind{
				{
					Kind: types.KindRecordingEncryption,
				},
			},
		})
		if err != nil {
			w.logger.ErrorContext(ctx, "failed to create watcher, retrying", "error", err)
			if !shouldRetryAfterJitterFn() {
				return nil
			}
			continue
		}
		defer watch.Close()

	HandleEvents:
		for {
			err := w.handleRecordingEncryptionChange(ctx)
			if err != nil {
				w.logger.ErrorContext(ctx, "failure while resolving recording encryption state", "error", err)
				if !shouldRetryAfterJitterFn() {
					return nil
				}
				continue

			}

			select {
			case ev := <-watch.Events():
				if ev.Type != types.OpPut {
					continue
				}
			case <-watch.Done():
				if err := watch.Error(); err == nil {
					return nil
				}

				w.logger.ErrorContext(ctx, "watcher failed, retrying", "error", err)
				if !shouldRetryAfterJitterFn() {
					return nil
				}
				break HandleEvents
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// this helper handles reacting to individual Put events on the RecordingEncryption resource and updates the
// SessionRecordingConfig with the results, if necessary
func (w *Watcher) handleRecordingEncryptionChange(ctx context.Context) error {
	recConfig, err := w.clusterConfig.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err, "fetching recording config")
	}

	if !recConfig.GetEncrypted() {
		w.logger.DebugContext(ctx, "session recording encryption disabled, skip resolving keys")
		return nil
	}

	_, err = w.resolver.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		if !recConfig.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys)) {
			return nil
		}

		_, err = w.clusterConfig.UpdateSessionRecordingConfig(ctx, recConfig)
		return trace.Wrap(err, "updating encryption keys")
	})

	if err != nil {
		return trace.Wrap(err, "resolving recording encryption")
	}

	return nil
}

// getAgeEncryptionKeys returns an iterator of AgeEncryptionKeys from a list of WrappedKeys. This is for use in
// populating the EncryptionKeys field of SessionRecordingConfigStatus.
func getAgeEncryptionKeys(keys []*recordingencryptionv1.WrappedKey) iter.Seq[*types.AgeEncryptionKey] {
	return func(yield func(*types.AgeEncryptionKey) bool) {
		for _, key := range keys {
			if !yield(&types.AgeEncryptionKey{
				PublicKey: key.RecordingEncryptionPair.PublicKey,
			}) {
				return
			}
		}
	}
}
