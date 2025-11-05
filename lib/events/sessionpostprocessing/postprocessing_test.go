/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package sessionpostprocessing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/events/sessionpostprocessing"
	"github.com/gravitational/teleport/lib/session"
)

func TestSessionPostProcessor(t *testing.T) {
	sessionID := session.ID(uuid.NewString())

	metadataProvider := recordingmetadata.NewProvider()
	recorderMetadata := &fakeRecordingMetadata{}
	recorderMetadata.On(
		"ProcessSessionRecording",
		mock.Anything,
		sessionID,
		mock.Anything,
	).
		Return(nil).Once()
	metadataProvider.SetService(recorderMetadata)

	summarizerProvider := summarizer.NewSessionSummarizerProvider()
	sessionSummarizer := &fakeSummarizer{}
	sessionSummarizer.On(
		"SummarizeSSH",
		mock.Anything,
		mock.Anything,
	).Return(nil).Once()
	summarizerProvider.SetSummarizer(sessionSummarizer)

	events := eventstest.GenerateTestSession(eventstest.SessionParams{
		UserName:  "alice",
		SessionID: string(sessionID),
		ServerID:  "testcluster",
		PrintData: []string{"net", "stat"},
	})

	cfg := sessionpostprocessing.Config{
		SessionEnd:                events[len(events)-1],
		RecordingMetadataProvider: metadataProvider,
		SessionSummarizerProvider: summarizerProvider,
		SessionID:                 sessionID,
	}

	err := sessionpostprocessing.Process(t.Context(), cfg)
	require.NoError(t, err)

	recorderMetadata.AssertExpectations(t)
	sessionSummarizer.AssertExpectations(t)
}

type fakeRecordingMetadata struct {
	mock.Mock
}

func (f *fakeRecordingMetadata) ProcessSessionRecording(ctx context.Context, sessionID session.ID, duration time.Duration) error {
	args := f.Called(ctx, sessionID, duration)
	return args.Error(0)
}

type fakeSummarizer struct {
	mock.Mock
}

func (f *fakeSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *apievents.SessionEnd) error {
	args := f.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (f *fakeSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *apievents.DatabaseSessionEnd) error {
	args := f.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (f *fakeSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	args := f.Called(ctx, sessionID)
	return args.Error(0)
}
