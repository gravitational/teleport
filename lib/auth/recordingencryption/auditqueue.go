// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	"filippo.io/age"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

const (
	auditQueueSealerWatchRetryInterval = time.Second
	auditQueueSealerRefreshTimeout     = 30 * time.Second
	auditQueueSealerWatchRetryMax      = time.Minute
)

// SessionRecordingConfigWatcher reads the session recording config and
// subscribes to changes to it.
type SessionRecordingConfigWatcher interface {
	SessionRecordingConfigGetter

	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// AuditQueueSealer encrypts audit queue events.
type AuditQueueSealer struct {
	client       SessionRecordingConfigWatcher
	retry        retryutils.Retry
	loopCtx      context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	initialized  bool
	fetchFailing bool
	encrypted    bool
	recipients   []age.Recipient
}

// NewAuditQueueSealer returns an AuditQueueSealer.
func NewAuditQueueSealer(ctx context.Context, client SessionRecordingConfigWatcher) (*AuditQueueSealer, error) {
	if client == nil {
		return nil, trace.BadParameter("SessionRecordingConfigWatcher is required for AuditQueueSealer")
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(auditQueueSealerWatchRetryInterval),
		Driver: retryutils.NewExponentialDriver(auditQueueSealerWatchRetryInterval),
		Max:    auditQueueSealerWatchRetryMax,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sealer := &AuditQueueSealer{
		client: client,
		retry:  retry,
	}
	if err := sealer.refresh(ctx); err != nil {
		return nil, trace.Wrap(err, "reading session recording config for audit queue encryption")
	}

	sealer.loopCtx, sealer.cancel = context.WithCancel(context.Background())
	sealer.wg.Go(sealer.watchLoop)
	return sealer, nil
}

// Close stops the background config watcher.
func (s *AuditQueueSealer) Close() error {
	s.cancel()
	s.wg.Wait()
	return nil
}

type encryptionState struct {
	encrypted  bool
	recipients []age.Recipient
}

func (s *AuditQueueSealer) encryptionState() (encryptionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return encryptionState{}, trace.Errorf("audit queue sealer has not resolved the encryption keys")
	}
	return encryptionState{
		encrypted:  s.encrypted,
		recipients: s.recipients,
	}, nil
}

// Seal encrypts a byte slice to all of the recipients.
// It returns the encrypted bytes, a bool to say whether the data was encrypted
// or not, and an error. When session recording encryption is disabled, the
// plaintext is returned unchanged.
func (s *AuditQueueSealer) Seal(ctx context.Context, plaintext []byte) ([]byte, bool, error) {
	state, err := s.encryptionState()
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if !state.encrypted {
		return plaintext, false, nil
	}

	var sealed bytes.Buffer
	w, err := age.Encrypt(&sealed, state.recipients...)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, false, trace.Wrap(err)
	}
	if err := w.Close(); err != nil {
		return nil, false, trace.Wrap(err)
	}
	return sealed.Bytes(), true, nil
}

func (s *AuditQueueSealer) watchLoop() {
	for {
		err := s.watch()
		if s.loopCtx.Err() != nil {
			return
		}
		if err != nil {
			s.warnOnFailureStreak(err)
		}

		select {
		case <-s.loopCtx.Done():
			return
		case <-s.retry.After():
			s.retry.Inc()
		}
	}
}

func (s *AuditQueueSealer) watch() error {
	watcher, err := s.client.NewWatcher(s.loopCtx, types.Watch{
		Name:  "audit-queue-sealer",
		Kinds: []types.WatchKind{{Kind: types.KindSessionRecordingConfig}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	select {
	case <-s.loopCtx.Done():
		return nil
	case <-watcher.Done():
		return trace.Wrap(watcher.Error())
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v", event.Type)
		}
	}
	s.retry.Reset()

	if err := s.refreshWithTimeout(); err != nil {
		s.warnOnFailureStreak(err)
	}

	for {
		select {
		case <-s.loopCtx.Done():
			return nil
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		case event := <-watcher.Events():
			if err := s.handleEvent(event); err != nil {
				s.warnOnFailureStreak(err)
			}
		}
	}
}

func (s *AuditQueueSealer) handleEvent(event types.Event) error {
	switch event.Type {
	case types.OpPut:
		config, ok := event.Resource.(types.SessionRecordingConfig)
		if !ok {
			return trace.BadParameter("expected SessionRecordingConfig resource, got %T", event.Resource)
		}
		return trace.Wrap(s.applyConfig(config))
	case types.OpDelete:
		return trace.Wrap(s.refreshWithTimeout())
	default:
		return nil
	}
}

func (s *AuditQueueSealer) refreshWithTimeout() error {
	ctx, cancel := context.WithTimeout(s.loopCtx, auditQueueSealerRefreshTimeout)
	defer cancel()
	return trace.Wrap(s.refresh(ctx))
}

func (s *AuditQueueSealer) refresh(ctx context.Context) error {
	config, err := s.client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.applyConfig(config))
}

func (s *AuditQueueSealer) applyConfig(config types.SessionRecordingConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encrypted, recipients, err := parseRecipients(config)
	if err != nil {
		return trace.Wrap(err)
	}

	s.initialized = true
	s.fetchFailing = false
	s.encrypted = encrypted
	s.recipients = recipients
	return nil
}

func (s *AuditQueueSealer) warnOnFailureStreak(cause error) {
	if s.loopCtx.Err() != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fetchFailing {
		return
	}
	s.fetchFailing = true
	slog.WarnContext(s.loopCtx,
		"Failed to refresh audit queue encryption keys. Continuing with the last known keys.",
		"error", cause,
	)
}

func parseRecipients(config types.SessionRecordingConfig) (bool, []age.Recipient, error) {
	if !config.GetEncrypted() {
		return false, nil, nil
	}

	keys := config.GetEncryptionKeys()
	if len(keys) == 0 {
		return false, nil, trace.NotFound("session recording encryption is enabled but no encryption keys are available")
	}
	recipients := make([]age.Recipient, 0, len(keys))
	for _, key := range keys {
		recipient, err := ParseAuditQueueRecipient(key.PublicKey)
		if err != nil {
			return false, nil, trace.Wrap(err, "parsing session recording encryption key")
		}
		recipients = append(recipients, recipient)
	}
	return true, recipients, nil
}
