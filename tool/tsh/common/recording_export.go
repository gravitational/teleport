/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/tool/tsh/common/internal/recordingexport"
)

const (
	framesPerSecond  = 30
	frameDelayMillis = float64(1000) / framesPerSecond
)

func onExportRecording(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	filenamePrefix := cf.SessionID
	if cf.OutFile != "" {
		// trim the extension if it was provided (we'll add it later on)
		filenamePrefix = strings.TrimSuffix(strings.TrimSuffix(cf.OutFile, ".avi"), ".AVI")
		filenamePrefix = strings.TrimSuffix(strings.TrimSuffix(filenamePrefix, ".mp4"), ".MP4")
	}

	exporter := &desktopRecordingExporter{
		ss:  clusterClient.AuthClient,
		sid: session.ID(cf.SessionID),
	}

	meta, err := exporter.getSessionMetadata(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set up the encoder which writes the output video.
	// Use ffmpeg if it's installed on the system, otherwise fall back to a Go-based AVI encoder.
	var encoder videoEncoder

	if cf.Format == "auto" {
		// check for ffmpeg
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: ffmpeg is not installed, falling back to legacy AVI encoder.")
			cf.Format = "avi"
		} else {
			cf.Format = "ffmpeg"
		}
	}
	switch cf.Format {
	case "ffmpeg":
		encoder, err = recordingexport.NewFFMPEGEncoder(filenamePrefix, framesPerSecond)
		if err != nil {
			return trace.Wrap(err, "creating ffmpeg encoder")
		}
	case "avi":
		encoder = recordingexport.NewAVIEncoder(filenamePrefix, int32(meta.maxWidth), int32(meta.maxHeight), framesPerSecond)
	}
	defer func() {
		if err := encoder.Close(); err != nil {
			logger.WarnContext(cf.Context, "failed to close encoder", "error", err)
		}
	}()

	// Set up the decoder to decode session data. It's most likely a RemoteFX session recording,
	// but very old recordings might require a PNG decoder.
	var decoder imageDecoder
	if meta.isRemoteFX {
		decoder, err = recordingexport.NewRemoteFXDecoder(meta.maxWidth, meta.maxHeight)
		if err != nil {
			return trace.Wrap(err, "creating RemoteFX decoder")
		}
	} else {
		decoder = recordingexport.NewPNGDecoder(int(meta.maxWidth), int(meta.maxHeight))
	}

	// Encode the video.
	// TODO(zmb3): add progress indicator
	if _, err := exporter.run(cf.Context, meta, decoder, encoder); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type desktopRecordingExporter struct {
	ss  events.SessionStreamer
	sid session.ID
}

type recordingMetadata struct {
	totalEvents uint32
	maxWidth    uint32
	maxHeight   uint32
	isRemoteFX  bool
}

// videoEncoder in an abstraction over the code that produces a video file
// based on the contents of a desktop session recording.
type videoEncoder interface {
	EmitFrames(img image.Image, count int) error
	// Close releases any resources held by the encoder.
	// Must be idempotent.
	Close() error
}

// imageDecoder decodes image data from a desktop session recording
type imageDecoder interface {
	ClearScreen()
	UpdateScreen(tdp.Message) error
	Image() image.Image
	Close() error
}

func (d *desktopRecordingExporter) run(
	ctx context.Context,
	meta *recordingMetadata,
	decoder imageDecoder,
	encoder videoEncoder,
) (frames int, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var frameCount int
	var gotScreenData bool
	lastEmitted := int64(-1)

	evts, errs := d.ss.StreamSessionEvents(ctx, d.sid, 0)
loop:
	for {
		select {
		case err := <-errs:
			if err := encoder.Close(); err != nil {
				logger.WarnContext(ctx, "failed to close encoder", "error", err)
			}
			return frameCount, trace.Wrap(err)
		case <-ctx.Done():
			if err := encoder.Close(); err != nil {
				logger.WarnContext(ctx, "failed to close encoder", "error", err)
			}
			return frameCount, trace.Wrap(ctx.Err())
		case evt, more := <-evts:
			if !more {
				logger.WarnContext(ctx, "reached end of stream before seeing session end event")
				break loop
			}

			switch evt := evt.(type) {
			case *apievents.WindowsDesktopSessionStart:
			case *apievents.WindowsDesktopSessionEnd:
				break loop

			case *apievents.DesktopRecording:
				msg, err := tdp.Decode(evt.Message)
				if err != nil {
					logger.WarnContext(ctx, "failed to decode desktop recording message", "error", err)
					break loop
				}

				switch msg := msg.(type) {
				case tdp.ClientScreenSpec:
					decoder.ClearScreen()
				case tdp.RDPFastPathPDU:
					gotScreenData = true
					if err := decoder.UpdateScreen(msg); err != nil {
						return frameCount, trace.Wrap(err)
					}
				case tdp.PNGFrame, tdp.PNG2Frame:
					gotScreenData = true
					decoder.UpdateScreen(msg)
				}

				// if it's the very first screen fragment, mark the time and continue
				// (no need to emit a frame yet)
				if lastEmitted == -1 {
					lastEmitted = evt.DelayMilliseconds
					continue loop
				}

				// If we haven't received any image data yet there's nothing to emit.
				if !gotScreenData {
					continue loop
				}

				// emit a frame if there's been enough of a time lapse between last event
				delta := evt.DelayMilliseconds - lastEmitted
				framesToEmit := int64(float64(delta) / frameDelayMillis)
				if framesToEmit > 0 {
					logger.DebugContext(ctx, "emitting frames",
						"last_event_ms", delta,
						"frames_to_emit", framesToEmit,
					)
					if err := encoder.EmitFrames(decoder.Image(), int(framesToEmit)); err != nil {
						// TODO: close encoder
						return frameCount, trace.Wrap(err)
					}
					frameCount += int(framesToEmit)
				}
				lastEmitted = evt.DelayMilliseconds
			}
		}
	}

	if err := encoder.Close(); err != nil {
		return frameCount, trace.Wrap(err, "closing encoder")
	}

	return frameCount, nil
}

// getSessionMetadata pre-processes the session recording in order to identify
// metadata necessary for the export.
func (d *desktopRecordingExporter) getSessionMetadata(ctx context.Context) (*recordingMetadata, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var remoteFX bool
	total, width, height := uint32(0), uint32(0), uint32(0)

	evts, errs := d.ss.StreamSessionEvents(ctx, d.sid, 0)
loop:
	for {
		select {
		case err := <-errs:
			return nil, trace.Wrap(err, "failed to process session")
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case evt, ok := <-evts:
			if !ok {
				break loop
			}

			switch evt.(type) {
			case *apievents.SessionStart,
				*apievents.AppSessionStart,
				*apievents.MCPSessionStart,
				*apievents.DatabaseSessionStart:
				return nil, trace.BadParameter("only desktop recordings can be exported")
			}

			total++
			dr, ok := evt.(*apievents.DesktopRecording)
			if !ok {
				continue
			}
			msg, err := tdp.Decode(dr.Message)
			if err != nil {
				return nil, trace.Wrap(err, "recording includes invalid data")
			}

			switch msg := msg.(type) {
			case tdp.ClientScreenSpec:
				height = max(height, msg.Height)
				width = max(width, msg.Width)
			case tdp.RDPFastPathPDU:
				remoteFX = true
			}
		}
	}
	return &recordingMetadata{totalEvents: total, maxWidth: width, maxHeight: height, isRemoteFX: remoteFX}, nil
}
