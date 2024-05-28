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
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/gravitational/trace"
	"github.com/icza/mjpeg"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
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

	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	authClient := proxyClient.CurrentCluster()
	defer authClient.Close()

	filenamePrefix := cf.SessionID
	if cf.OutFile != "" {
		// trim the extension if it was provided (we'll add it later on)
		filenamePrefix = strings.TrimSuffix(
			strings.TrimSuffix(cf.OutFile, ".avi"), ".AVI")
	}

	_, err = writeMovie(cf.Context, authClient, session.ID(cf.SessionID), filenamePrefix, fmt.Printf, tc.Config.WebProxyAddr)
	return trace.Wrap(err)
}

func makeAVIFileName(prefix string, currentFile int) string {
	if currentFile == 0 {
		return prefix + ".avi"
	}

	return fmt.Sprintf("%v-%d.avi", prefix, currentFile)
}

// writeMovie writes the events for the specified session into one or more movie files
// beginning with the specified prefix. It returns the number of frames that were written and an error.
func writeMovie(ctx context.Context, ss events.SessionStreamer, sid session.ID, prefix string, write func(format string, args ...any) (int, error), webProxyAddr string) (frames int, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var screen *image.NRGBA
	var movie mjpeg.AviWriter

	lastEmitted := int64(-1)
	buf := new(bytes.Buffer)

	var frameCount, fileCount int
	var width, height int32
	currentFilename := makeAVIFileName(prefix, fileCount)

	evts, errs := ss.StreamSessionEvents(ctx, sid, 0)
	fastPathReceived := false
loop:
	for {
		select {
		case err := <-errs:
			if movie != nil {
				if err := movie.Close(); err == nil && write != nil && frames > 0 {
					write("wrote %v\n", currentFilename)
				}
			}
			return frameCount, trace.Wrap(err)
		case <-ctx.Done():
			if movie != nil {
				if err := movie.Close(); err == nil && write != nil && frames > 0 {
					write("wrote %v\n", currentFilename)
				}
			}
			return frameCount, ctx.Err()
		case evt, more := <-evts:
			if !more {
				log.Warnln("reached end of stream before seeing session end event")
				break loop
			}

			switch evt := evt.(type) {
			case *apievents.WindowsDesktopSessionStart:
			case *apievents.WindowsDesktopSessionEnd:
				if !fastPathReceived {
					break loop
				}
				if movie != nil {
					movie.Close()
					os.Remove(currentFilename)
				}
				url := fmt.Sprintf("https://%s/web/cluster/%s/session/%s?recordingType=desktop&durationMs=%d",
					webProxyAddr,
					evt.ClusterName,
					evt.SessionID,
					evt.EndTime.Sub(evt.StartTime).Milliseconds())
				return frameCount, trace.BadParameter("this session can't be exported, please visit %s to view it", url)
			case *apievents.SessionStart:
				return frameCount, trace.BadParameter("only desktop recordings can be exported")
			case *apievents.DesktopRecording:
				msg, err := tdp.Decode(evt.Message)
				if err != nil {
					log.Warnf("failed to decode desktop recording message: %v", err)
					break loop
				}

				switch msg := msg.(type) {
				case tdp.RDPFastPathPDU:
					fastPathReceived = true
				case tdp.ClientScreenSpec:
					if screen != nil {
						return frameCount, trace.BadParameter("invalid recording: received multiple screen specs")
					}
					// Use the dimensions in the ClientScreenSpec to allocate
					// our virtual canvas and video file.
					// Note: this works because we don't currently support resizing
					// the window during a session. If this changes, we'd have to
					// find the maximum window size first.
					log.Debugf("allocating %dx%d screen", msg.Width, msg.Height)
					width, height = int32(msg.Width), int32(msg.Height)
					screen = image.NewNRGBA(image.Rectangle{
						Min: image.Pt(0, 0),
						Max: image.Pt(int(msg.Width), int(msg.Height)),
					})

					movie, err = mjpeg.New(currentFilename, width, height, framesPerSecond)
					if err != nil {
						return frameCount, trace.Wrap(err)
					}

				case tdp.PNGFrame, tdp.PNG2Frame:
					if screen == nil {
						return frameCount, trace.BadParameter("this session is missing required start metadata")
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

				// emit a frame if there's been enough of a time lapse between last event
				delta := evt.DelayMilliseconds - lastEmitted
				framesToEmit := int64(float64(delta) / frameDelayMillis)
				if framesToEmit > 0 {
					log.Debugf("%dms since last frame, emitting %d frames", delta, framesToEmit)
					buf.Reset()
					if err := jpeg.Encode(buf, screen, nil); err != nil {
						return frameCount, trace.Wrap(err)
					}
					for i := 0; i < int(framesToEmit); i++ {
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
							movie, err = mjpeg.New(currentFilename, width, height, framesPerSecond)
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
				log.Debugf("got unexpected audit event %T", evt)
			}
		}
	}

	// if we received a session start event but the context is canceled
	// before we received the screen dimensions, then there's no movie to close
	if movie == nil {
		return 0, ctx.Err()
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
