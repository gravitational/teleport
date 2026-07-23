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
	"github.com/jonboulle/clockwork"
)

const (
	auditQueueSealerRefreshInterval = time.Minute
	auditQueueSealerRetryInterval   = 10 * time.Second
	auditQueueSealerRefreshTimeout  = 30 * time.Second
)

// AuditQueueSealer encrypts audit queue events.
type AuditQueueSealer struct {
	srcGetter SessionRecordingConfigGetter
	clock     clockwork.Clock
	loopCtx   context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	mu         sync.Mutex
	ready      bool
	encrypted  bool
	recipients []age.Recipient
}

// NewAuditQueueSealer returns an AuditQueueSealer.
func NewAuditQueueSealer(ctx context.Context, srcGetter SessionRecordingConfigGetter) (*AuditQueueSealer, error) {
	if srcGetter == nil {
		return nil, trace.BadParameter("SessionRecordingConfigGetter is required for AuditQueueSealer")
	}

	sealer := &AuditQueueSealer{
		srcGetter: srcGetter,
		clock:     clockwork.NewRealClock(),
	}
	if err := sealer.refreshOnce(ctx); err != nil {
		return nil, trace.Wrap(err, "reading session recording config for audit queue encryption")
	}

	sealer.loopCtx, sealer.cancel = context.WithCancel(context.Background())
	sealer.wg.Go(sealer.refreshLoop)
	return sealer, nil
}

// Close stops the background key refresh.
func (s *AuditQueueSealer) Close() error {
	s.cancel()
	s.wg.Wait()
	return nil
}

type encryptionState struct {
	ready      bool
	encrypted  bool
	recipients []age.Recipient
}

func (s *AuditQueueSealer) encryptionState() encryptionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return encryptionState{
		ready:      s.ready,
		encrypted:  s.encrypted,
		recipients: s.recipients,
	}
}

// Seal encrypts a byte slice to all of the recipients.
// It returns the encrypted bytes, a bool to say whether the data was encrypted
// or not, and an error. When session recording encryption is disabled, the
// plaintext is returned unchanged.
func (s *AuditQueueSealer) Seal(ctx context.Context, plaintext []byte) ([]byte, bool, error) {
	state := s.encryptionState()

	if !state.ready {
		return nil, false, trace.Errorf("audit queue sealer has not resolved the encryption keys")
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

func (s *AuditQueueSealer) refreshLoop() {
	timer := s.clock.NewTimer(auditQueueSealerRefreshInterval)
	defer timer.Stop()
	for {
		select {
		case <-s.loopCtx.Done():
			return
		case <-timer.Chan():
		}

		interval := auditQueueSealerRefreshInterval
		refreshCtx, cancel := context.WithTimeout(s.loopCtx, auditQueueSealerRefreshTimeout)
		err := s.refreshOnce(refreshCtx)
		cancel()
		if err != nil {
			if s.loopCtx.Err() == nil {
				slog.WarnContext(s.loopCtx,
					"Failed to refresh audit queue encryption keys. Continuing with the last known keys.",
					"error", err,
				)
			}
			interval = auditQueueSealerRetryInterval
		}
		timer.Reset(interval)
	}
}

func (s *AuditQueueSealer) refreshOnce(ctx context.Context) error {
	encrypted, recipients, err := s.fetchRecipients(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	s.encrypted = encrypted
	s.recipients = recipients
	return nil
}

func (s *AuditQueueSealer) fetchRecipients(ctx context.Context) (bool, []age.Recipient, error) {
	config, err := s.srcGetter.GetSessionRecordingConfig(ctx)
	if err != nil {
		return false, nil, trace.Wrap(err)
	}

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
