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
	switch o := cfg.SessionEnd.(type) {
	case *apievents.SessionEnd:
		if err := summarizer.SummarizeSSH(ctx, o); err != nil {
			summarizerErr = trace.Wrap(err, "failed to summarize upload")
		}
		metadataSvc := cfg.RecordingMetadataProvider.Service()
		if !o.EndTime.IsZero() && !o.StartTime.IsZero() {
			duration := o.EndTime.Sub(o.StartTime)
			if err := metadataSvc.ProcessSessionRecording(ctx, cfg.SessionID, duration); err != nil {
				metadataErr = trace.Wrap(err, "failed to process session recording metadata")
			}
		}
	case *apievents.DatabaseSessionEnd:
		if err := summarizer.SummarizeDatabase(ctx, o); err != nil {
			summarizerErr = trace.Wrap(err, "failed to summarize upload")
		}
	}
	return trace.NewAggregate(summarizerErr, metadataErr)
}
