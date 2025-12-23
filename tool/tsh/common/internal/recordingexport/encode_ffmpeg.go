/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package recordingexport

import (
	"bytes"
	"image"
	"io"
	"os/exec"
	"strconv"

	"github.com/gravitational/trace"
	"golang.org/x/image/bmp"
)

// NewFFMPEGEncoder encodes videos using the system-installed ffmpeg binary.
func NewFFMPEGEncoder(outputPrefix string, fps int) (*FFMPEGEncoder, error) {
	framesPerSecond := strconv.Itoa(fps)
	cmd := exec.Command("ffmpeg",
		"-y", // overwrite output file without asking

		// Input args:
		"-f", "image2pipe", // format: image data from a pipe (not files on disk)
		"-framerate", framesPerSecond, // input isn't a video so we must specify the framerate
		"-vcodec", "bmp", // codec: each image from the pipe is a bitmap image
		"-i", "-", // read from stdin

		// Output args:
		"-c:v", "libx264", // codec: encode video w/ libx264
		"-r", framesPerSecond,
		"-preset", "veryfast", // a good tradeoff between encoding speed and output size
		"-pix_fmt", "yuv420p", // use a commonly-supported pixel format
		"-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2", // H264 requires even dimensions, pad the output if necessary
		outputPrefix+".mp4", // output file
	)
	buf := new(bytes.Buffer)
	cmd.Stderr = buf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cmd.Start(); err != nil {
		return nil, trace.BadParameter("ffmpeg is not available on this system. Install ffmpeg or use --encoder=avi to use the built-in AVI encoder")
	}

	return &FFMPEGEncoder{
		cmd:    cmd,
		stdin:  stdin,
		stderr: buf,
	}, nil
}

// FFMPEGEncoder encodes video with ffmpeg. ffmpeg must be installed
// on the system.
type FFMPEGEncoder struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser

	stderr *bytes.Buffer
}

func (f *FFMPEGEncoder) EmitFrames(img image.Image, count int) error {
	if img == nil {
		return trace.BadParameter("no frame to export")
	}
	for range count {
		// ffmpeg supports a variety of input image formats.
		// We use bitmaps instead of something like PNG because it's essentially free to encode.
		// The bitmap encoding here just puts a header in front of the existing pixel data.
		if err := bmp.Encode(f.stdin, img); err != nil {
			// Close the input pipe. ffmpeg will finish any in-progress encoding and terminate.
			f.stdin.Close()
			f.cmd.Wait()

			f.stdin = nil
			f.cmd = nil

			return trace.Wrap(err, "encoding bitmap frame")
		}
	}
	return nil
}

func (f *FFMPEGEncoder) Close() error {
	if f.stdin != nil {
		f.stdin.Close()
	}
	var err error
	if f.cmd != nil {
		err = f.cmd.Wait()
	}

	f.stdin = nil
	f.cmd = nil

	if err != nil {
		return trace.Errorf("ffmpeg failed: %s", f.stderr.String())
	}

	return nil
}
