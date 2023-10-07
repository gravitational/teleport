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
	framesPerSecond    = 30
	frameDelayMillis   = float64(1000) / framesPerSecond
	fileSplitMegabytes = 3500
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
		fname = cf.SessionID
	} else {
		// remove suffix of file since we will add that automatically
		fname = strings.TrimSuffix(fname, ".avi")
		fname = strings.TrimSuffix(fname, ".AVI")

	}

	frames, numberOfFiles, err := writeMovie(cf.Context, authClient, session.ID(cf.SessionID), fname)
	// there may be a partial file, even if we encountered an error,
	// so we indicate to the user when we wrote something
	if frames > 0 {
		if numberOfFiles > 0 {
			fmt.Println("Multiple files were required due to size. Listed in order.")
		}
		for i := 0; i <= numberOfFiles; i++ {
			fmt.Printf("wrote recording to %v\n", getAVIFileName(fname, i))
		}

	}

	return trace.Wrap(err)
}

// returns the given file name given the file count and file name.
// count of 0 will get the filename with suffix of .avi.
// subsequent counts will get suffix of count.avi (ex: 0->file.avi, 1->file-1.avi, 2->file-2.avi)
func getAVIFileName(fname string, fileCount int) string {
	fileName := fname
	if fileCount == 0 {
		return fmt.Sprintf("%s.avi", fileName)
	}

	fileName = fmt.Sprintf("%s-%d.avi", fname, fileCount)
	return fileName
}

// writeMovie writes streams the events for the specified session into a movie file
// identified by fname. Due to restrictions on size of AVI files it will generate multiple files
// as each exceeds 3.5GB. It returns the number of frames that were written, the number of
// avi files and an error.
func writeMovie(ctx context.Context, ss events.SessionStreamer, sid session.ID, fname string) (int, int, error) {
	var screen *image.NRGBA
	var movie mjpeg.AviWriter
	setWidth := int32(0)
	setHeight := int32(0)

	lastEmitted := int64(-1)
	buf := new(bytes.Buffer)
	frameCount := 0
	numberOfFiles := 0
	currentFileName := getAVIFileName(fname, numberOfFiles)

	evts, errs := ss.StreamSessionEvents(ctx, sid, 0)
loop:
	for {
		select {
		case err := <-errs:
			if movie != nil {
				movie.Close()
			}
			return frameCount, numberOfFiles, trace.Wrap(err)
		case <-ctx.Done():
			if movie != nil {
				movie.Close()
			}
			return frameCount, numberOfFiles, ctx.Err()
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
				return frameCount, numberOfFiles, trace.BadParameter("only desktop recordings can be exported")
			case *apievents.DesktopRecording:
				msg, err := tdp.Decode(evt.Message)
				if err != nil {
					log.Warnf("failed to decode desktop recording message: %v", err)
					break loop
				}

				switch msg := msg.(type) {
				case tdp.ClientScreenSpec:
					if screen != nil {
						return frameCount, numberOfFiles, trace.BadParameter("invalid recording: received multiple screen specs")
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

					movie, err = mjpeg.New(currentFileName, int32(msg.Width), int32(msg.Height), framesPerSecond)
					setWidth = int32(msg.Width)
					setHeight = int32(msg.Height)
					if err != nil {
						return frameCount, numberOfFiles, trace.Wrap(err)
					}

				case tdp.PNGFrame, tdp.PNG2Frame:
					if screen == nil {
						return frameCount, numberOfFiles, trace.BadParameter("this session is missing required start metadata")
					}

					fragment, err := imgFromPNGMessage(msg)
					if err != nil {
						return frameCount, numberOfFiles, trace.WrapWithMessage(err, "couldn't decode PNG")
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

					// get the current size of the file writing to
					info, err := os.Stat(currentFileName)
					sizeOfFile := info.Size()
					log.Debugf("file size %v of %s", sizeOfFile, currentFileName)
					if err != nil {
						return frameCount, numberOfFiles, trace.Wrap(err)
					}
					// check if we've crossed the size to split into multiple files
					if sizeOfFile > (fileSplitMegabytes * 1048576) {
						log.Debugf("crossed file length to split, closing avi file %s", currentFileName)
						// close the current file
						err = movie.Close()
						if err != nil {
							return frameCount, numberOfFiles, trace.Wrap(err)
						}
						// start a new avi file
						numberOfFiles++
						currentFileName = getAVIFileName(fname, numberOfFiles)
						log.Debugf("starting new avi file %s", currentFileName)
						movie, err = mjpeg.New(currentFileName, setWidth, setHeight, framesPerSecond)
						if err != nil {
							return frameCount, numberOfFiles, trace.Wrap(err)
						}

					}

					log.Debugf("%dms since last frame, emitting %d frames", delta, framesToEmit)
					buf.Reset()
					if err := jpeg.Encode(buf, screen, nil); err != nil {
						return frameCount, numberOfFiles, trace.Wrap(err)
					}
					for i := 0; i < int(framesToEmit); i++ {
						if err := movie.AddFrame(buf.Bytes()); err != nil {
							return frameCount, numberOfFiles, trace.Wrap(err)
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
		return 0, 0, trace.BadParameter("operation canceled")
	}

	return frameCount, numberOfFiles, trace.Wrap(movie.Close())
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
