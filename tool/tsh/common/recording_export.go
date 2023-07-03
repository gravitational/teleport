/*
Copyright 2023 Gravitational, Inc.

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

package common

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"

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

	fname := cf.OutFile
	if fname == "" {
		fname = cf.SessionID + ".avi"
	}

	frames, err := writeMovie(cf.Context, authClient, session.ID(cf.SessionID), fname)
	// there may be a partial file, even if we encountered an error,
	// so we indicate to the user when we wrote something
	if frames > 0 {
		fmt.Printf("wrote recording to %v\n", fname)
	}

	return trace.Wrap(err)
}

// writeMovie writes streams the events for the specified session into a movie file
// identified by fname. It returns the number of frames that were written and an error.
func writeMovie(ctx context.Context, ss events.SessionStreamer, sid session.ID, fname string) (int, error) {
	var screen *image.NRGBA
	var movie mjpeg.AviWriter

	lastEmitted := int64(-1)
	buf := new(bytes.Buffer)
	frameCount := 0

	evts, errs := ss.StreamSessionEvents(ctx, sid, 0)
loop:
	for {
		select {
		case err := <-errs:
			if movie != nil {
				movie.Close()
			}
			return frameCount, trace.Wrap(err)
		case <-ctx.Done():
			if movie != nil {
				movie.Close()
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
				break loop
			case *apievents.SessionStart:
				return frameCount, trace.BadParameter("only desktop recordings can be exported")
			case *apievents.DesktopRecording:
				msg, err := tdp.Decode(evt.Message)
				if err != nil {
					log.Warnf("failed to decode desktop recording message: %v", err)
					break loop
				}

				switch msg := msg.(type) {
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
					screen = image.NewNRGBA(image.Rectangle{
						Min: image.Pt(0, 0),
						Max: image.Pt(int(msg.Width), int(msg.Height)),
					})

					movie, err = mjpeg.New(fname, int32(msg.Width), int32(msg.Height), framesPerSecond)
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
						if err := movie.AddFrame(buf.Bytes()); err != nil {
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

	return frameCount, trace.Wrap(movie.Close())
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
