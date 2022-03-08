/*
Copyright 2022 Gravitational, Inc.

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

package events

import (
	"context"
	"testing"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestUploadCompleterCompletesEmptyUploads verifies that the upload completer
// completes uploads that have no parts. This ensures that we don't leave empty
// directories behind.
func TestUploadCompleterCompletesEmptyUploads(t *testing.T) {
	clock := clockwork.NewFakeClock()
	mu := NewMemoryUploader()
	mu.Clock = clock

	log := &MockAuditLog{}

	uc, err := NewUploadCompleter(UploadCompleterConfig{
		Unstarted:   true,
		Uploader:    mu,
		AuditLog:    log,
		GracePeriod: 2 * time.Hour,
	})
	require.NoError(t, err)

	upload, err := mu.CreateUpload(context.Background(), session.NewID())
	require.NoError(t, err)
	clock.Advance(3 * time.Hour)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)

	require.True(t, mu.uploads[upload.ID].completed)
}

type MockAuditLog struct {
	DiscardAuditLog

	emitter       MockEmitter
	sessionEvents []EventFields
}

func (m *MockAuditLog) GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]EventFields, error) {
	return m.sessionEvents, nil
}

func (m *MockAuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return m.emitter.EmitAuditEvent(ctx, event)
}
