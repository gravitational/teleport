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
package sessionpostprocessing

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/session"
)

// Config is the configuration for the session post-processor.
type Config struct {
	// SessionSummarizerProvider is a provider of the session summarizer service.
	// It can be nil or provide a nil summarizer if summarization is not needed.
	// The summarizer itself summarizes session recordings.
	SessionSummarizerProvider *summarizer.SessionSummarizerProvider
	// RecordingMetadataProvider is a provider of the recording metadata service.
	RecordingMetadataProvider *recordingmetadata.Provider
	// SessionEnd is the session end event to process.
	SessionEnd apievents.AuditEvent
	// SessionID is the ID of the session being processed.
	SessionID session.ID
}

// Process processes session end events after the session recording upload is complete.
// It summarizes the session recording and processes the recording metadata.
func Process(ctx context.Context, cfg Config) error {
	switch {
	case cfg.SessionSummarizerProvider == nil:
		return trace.BadParameter("session summarizer provider is not set")
	case cfg.RecordingMetadataProvider == nil:
		return trace.BadParameter("recording metadata provider is not set")
	case cfg.SessionEnd == nil:
		return trace.BadParameter("session end event is not set")
	case cfg.SessionID == "":
		return trace.BadParameter("session ID is not set")
	}

	var summarizerErr error
	var metadataErr error
	summarizer := cfg.SessionSummarizerProvider.SessionSummarizer()

	// A recovered session can be missing a time bound, leaving no interval to generate metadata over.
	processMetadata := func(sessionType recordingmetadata.SessionType, startTime, endTime time.Time) {
		if startTime.IsZero() || endTime.IsZero() {
			return
		}
		metadataSvc := cfg.RecordingMetadataProvider.Service()
		if err := metadataSvc.ProcessSessionRecording(ctx, cfg.SessionID, sessionType, startTime, endTime.Sub(startTime)); err != nil {
			metadataErr = trace.Wrap(err, "failed to process session recording metadata")
		}
	}

	switch o := cfg.SessionEnd.(type) {
	case *apievents.SessionEnd:
		if err := summarizer.SummarizeSSH(ctx, o); err != nil {
			summarizerErr = trace.Wrap(err, "failed to summarize upload")
		}
		processMetadata(recordingmetadata.SessionTypeTTY, o.StartTime, o.EndTime)
	case *apievents.DatabaseSessionEnd:
		if err := summarizer.SummarizeDatabase(ctx, o); err != nil {
			summarizerErr = trace.Wrap(err, "failed to summarize upload")
		}
	case *apievents.WindowsDesktopSessionEnd:
		if err := summarizer.SummarizeWindowsDesktop(ctx, o); err != nil {
			summarizerErr = trace.Wrap(err, "failed to summarize upload")
		}
		processMetadata(recordingmetadata.SessionTypeDesktop, o.StartTime, o.EndTime)
	case *apievents.LinuxDesktopSessionEnd:
		if err := summarizer.SummarizeLinuxDesktop(ctx, o); err != nil {
			summarizerErr = trace.Wrap(err, "failed to summarize upload")
		}
		processMetadata(recordingmetadata.SessionTypeDesktop, o.StartTime, o.EndTime)
	}
	return trace.NewAggregate(summarizerErr, metadataErr)
}
