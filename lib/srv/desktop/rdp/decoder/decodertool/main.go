// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"os/signal"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fname := os.Args[1]
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	pr := events.NewProtoReader(f, nil /* decrypter */)
	defer pr.Close()

	count := 0

	var (
		width, height uint32
		rdpDecoder    *decoder.Decoder
	)

	for {
		evt, err := pr.Read(ctx)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		count++

		recordingEvent, ok := evt.(*apievents.DesktopRecording)
		if !ok {
			log.Printf("skipping %v event (%d)", evt.GetType(), count)
			continue
		}

		msg, err := tdp.Decode(recordingEvent.Message)
		if err != nil {
			log.Println("failed to decode message, skipping it")
			continue
		}

		switch msg := msg.(type) {
		case tdp.ConnectionActivated:
			width, height = uint32(msg.ScreenWidth), uint32(msg.ScreenHeight)

		case tdp.ClientScreenSpec:
			width, height = msg.Width, msg.Height
			if rdpDecoder != nil {
				rdpDecoder.Resize(uint16(msg.Width), uint16(msg.Height))
			}

		case tdp.RDPFastPathPDU:
			if rdpDecoder == nil {
				var err error
				rdpDecoder, err = decoder.New(uint16(width), uint16(height))
				if err != nil {
					log.Fatalf("couldn't create decoder: %v", err)
				}
			}

			rdpDecoder.Process([]byte(msg))

			if count%100 == 0 {
				writeImages(rdpDecoder.Image(), rdpDecoder.Thumbnail(1000, 600), count)
			}
		}
	}

	writeImages(rdpDecoder.Image(), rdpDecoder.Thumbnail(1000, 600), count)
}

func writeImages(img, thumb *image.RGBA, index int) error {
	if img == nil {
		return errors.New("nil image")
	}
	f, err := os.Create(fmt.Sprintf("screen-%08d.png", index))
	if err != nil {
		return err
	}

	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return err
	}

	t, err := os.Create(fmt.Sprintf("thumb-%08d.png", index))
	if err != nil {
		return err
	}

	defer t.Close()

	if err := png.Encode(t, thumb); err != nil {
		return err
	}

	return nil
}
