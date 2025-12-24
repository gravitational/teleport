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
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/gravitational/trace"
	"github.com/icza/mjpeg"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
	}

	exporter := &desktopRecordingExporter{
		ss:  clusterClient.AuthClient,
		sid: session.ID(cf.SessionID),
	}

	meta, err := exporter.getSessionMetadata(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = exporter.writeMovie(cf.Context, meta, filenamePrefix, fmt.Printf)
	return trace.Wrap(err)
}

func makeAVIFileName(prefix string, currentFile int) string {
	if currentFile == 0 {
		return prefix + ".avi"
	}

	return fmt.Sprintf("%v-%d.avi", prefix, currentFile)
}

type desktopRecordingExporter struct {
	ss  events.SessionStreamer
	sid session.ID
}

type recordingMetadata struct {
	totalEvents uint32
	maxWidth    uint32
	maxHeight   uint32
}

// getSessionMetadata pre-processes the session recording in order to identify
// metadata necessary for the export.
func (d *desktopRecordingExporter) getSessionMetadata(ctx context.Context) (*recordingMetadata, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
			total++
			dr, ok := evt.(*apievents.DesktopRecording)
			if !ok {
				continue
			}
			msg, err := tdp.Decode(dr.Message)
			if err != nil {
				return nil, trace.Wrap(err, "recording includes invalid data")
			}
			if resize, ok := msg.(tdp.ClientScreenSpec); ok {
				height = max(height, resize.Height)
				width = max(width, resize.Width)
			}
		}
	}
	return &recordingMetadata{totalEvents: total, maxWidth: width, maxHeight: height}, nil
}

// writeMovie writes the events for the specified session into one or more movie files
// beginning with the specified prefix. It returns the number of frames that were written and an error.
func (d *desktopRecordingExporter) writeMovie(
	ctx context.Context,
	meta *recordingMetadata,
	prefix string,
	write func(format string, args ...any) (int, error),
) (frames int, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var screen *image.NRGBA
	var fastPathDecoder *decoder.Decoder

	lastEmitted := int64(-1)
	buf := new(bytes.Buffer)

	var frameCount, fileCount int
	currentFilename := makeAVIFileName(prefix, fileCount)
	movie, err := mjpeg.New(currentFilename, int32(meta.maxWidth), int32(meta.maxHeight), framesPerSecond)
	if err != nil {
		return frameCount, trace.Wrap(err)
	}

	evts, errs := d.ss.StreamSessionEvents(ctx, d.sid, 0)

loop:
	for {
		select {
		case err := <-errs:
			if err := movie.Close(); err == nil && write != nil && frames > 0 {
				write("wrote %v\n", currentFilename)
			}
			return frameCount, trace.Wrap(err)
		case <-ctx.Done():
			if err := movie.Close(); err == nil && write != nil && frames > 0 {
				write("wrote %v\n", currentFilename)
			}
			return frameCount, ctx.Err()
		case evt, more := <-evts:
			if !more {
				logger.WarnContext(ctx, "reached end of stream before seeing session end event")
				break loop
			}

			switch evt := evt.(type) {
			case *apievents.WindowsDesktopSessionStart:
			case *apievents.WindowsDesktopSessionEnd:
				break loop
			case *apievents.SessionStart:
				return frameCount, trace.BadParameter("only desktop recordings can be exported")
			case *apievents.DesktopRecording:
				msg, err := tdp.Decode(evt.Message)
				if err != nil {
					logger.WarnContext(ctx, "failed to decode desktop recording message", "error", err)
					break loop
				}
				switch msg := msg.(type) {
				case tdp.ClientScreenSpec:
					// Clear the screen out. This is important to avoid visual artifacts if the screen
					// is made smaller during the session.
					if screen != nil {
						// Fill the screen with opaque black.
						draw.Draw(screen, screen.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
					} else if fastPathDecoder != nil {
						println("resizing rfx")
						fastPathDecoder.Resize(uint16(meta.maxWidth), uint16(meta.maxHeight))
					}

				case tdp.RDPFastPathPDU:
					if fastPathDecoder == nil {
						var err error
						fastPathDecoder, err = decoder.New(uint16(meta.maxWidth), uint16(meta.maxHeight))
						if err != nil {
							return frameCount, trace.Wrap(err)
						}
						defer fastPathDecoder.Release()
					}
					fastPathDecoder.Process(msg)

				case tdp.PNGFrame, tdp.PNG2Frame:
					if screen == nil {
						screen = image.NewNRGBA(image.Rectangle{
							Min: image.Pt(0, 0),
							Max: image.Pt(int(meta.maxWidth), int(meta.maxHeight)),
						})
					}

					fragment, err := imgFromPNGMessage(msg)
					if err != nil {
						return frameCount, trace.WrapWithMessage(err, "couldn't decode PNG")
					}

					// draw the fragment from this message on the screen
					draw.Draw(
						screen,
						rectFromPNGMessage(msg),
						fragment,
						fragment.Bounds().Min,
						draw.Src,
					)
				}

				// if it's the very first bitmap, mark the time and continue
				// (no need to emit a frame yet)
				if lastEmitted == -1 {
					lastEmitted = evt.DelayMilliseconds
					continue loop
				}

				// If we haven't received any image data yet there's nothing to emit.
				if screen == nil && fastPathDecoder == nil {
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
					buf.Reset()

					// The current state of the screen is different depending on whether
					// we're processing legacy PNG recordings or modern RemoteRF recordings.
					var image image.Image = screen
					if fastPathDecoder != nil {
						image = fastPathDecoder.Image() // TODO: nil check
					}

					if err := jpeg.Encode(buf, image, nil /* options */); err != nil {
						return frameCount, trace.Wrap(err)
					}
					for range int(framesToEmit) {
						err := movie.AddFrame(buf.Bytes())
						if errors.Is(err, mjpeg.ErrTooLarge) {
							// this file can't get any larger - time to open a new file
							if err := movie.Close(); err != nil {
								return frameCount, trace.WrapWithMessage(err, "failed to write partial recording")
							}
							if write != nil {
								write("wrote %v\n", currentFilename)
							}
							fileCount++
							currentFilename = makeAVIFileName(prefix, fileCount)
							movie, err = mjpeg.New(currentFilename, int32(meta.maxWidth), int32(meta.maxHeight), framesPerSecond)
							if err != nil {
								return frameCount, trace.Wrap(err)
							}

							// write the frame to the new file
							if err := movie.AddFrame(buf.Bytes()); err != nil {
								return frameCount, trace.Wrap(err)
							}
						} else if err != nil {
							return frameCount, trace.Wrap(err)
						}
						frameCount++
					}
					lastEmitted = evt.DelayMilliseconds
				}

			default:
				logger.DebugContext(ctx, "got unexpected audit event", "event", logutils.TypeAttr(evt))
			}
		}
	}

	err = movie.Close()
	if err == nil && write != nil {
		write("wrote %v\n", currentFilename)
	}

	return frameCount, trace.Wrap(err)
}

func imgFromPNGMessage(msg tdp.Message) (image.Image, error) {
	switch msg := msg.(type) {
	case tdp.PNG2Frame:
		return png.Decode(bytes.NewReader(msg.Data()))
	case tdp.PNGFrame:
		return msg.Img, nil
	default:
		// this should never happen based on what we pass at the call site
		return nil, trace.BadParameter("unsupported TDP message %T", msg)
	}
}

func rectFromPNGMessage(msg tdp.Message) image.Rectangle {
	switch msg := msg.(type) {
	case tdp.PNG2Frame:
		return image.Rect(
			// add one to bottom and right dimension, as RDP
			// bounds are inclusive
			int(msg.Left()), int(msg.Top()),
			int(msg.Right()+1), int(msg.Bottom()+1),
		)
	case tdp.PNGFrame:
		return msg.Img.Bounds()
	default:
		// this should never happen based on what we pass at the call site
		return image.Rectangle{}
	}
}
