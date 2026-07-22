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

package protocol

import (
	"bytes"
	"io"
	"net"
	"sync"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/datastax/go-cassandra-native-protocol/segment"
	"github.com/gravitational/trace"
)

// NewConn is used to create a new connection.
func NewConn(rawConn net.Conn) *Conn {
	return &Conn{
		Conn:        rawConn,
		frameCodec:  frame.NewRawCodec(),
		segmentCode: segment.NewCodec(),
	}
}

// Conn represent incoming client connection or ongoing connection to cassandra server.
// Reading and Writing Frames/Packages needs to be done sequential because Conn implementation
// is not thread safe. Conn package is used to intercept and preform custom Cassandra handshake
// and in case of connection incoming connection to provide ability to audit incoming client packages.
type Conn struct {
	net.Conn
	segmentCode       segment.Codec
	frameCodec        frame.RawCodec
	modernLayoutRead  bool
	modernLayoutWrite bool

	// selfContained is used to store frames that are self-contained.
	// allowing to read them before reading from the connection.
	selfContained []*frame.Frame
	mtx           sync.Mutex
}

// ReadPacket is used to read packet from the connection.
func (c *Conn) ReadPacket() (*Packet, error) {
	if fr := c.checkSelfContained(); fr != nil {
		return &Packet{frame: fr}, nil
	}
	var buff bytes.Buffer
	tr := io.TeeReader(c.Conn, &buff)

	fr, err := c.readWithModernLayout(tr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.maybeSwitchToModernLayout(fr)

	return &Packet{
		raw:   buff,
		frame: fr,
	}, nil
}

// WriteFrame is used to write frame to the connection.
func (c *Conn) WriteFrame(outgoing *frame.Frame) error {
	if err := c.writeFrameWithModernLayout(outgoing); err != nil {
		return trace.Wrap(err)
	}

	if startup, ok := outgoing.Body.Message.(*message.Startup); ok {
		compression := startup.GetCompression()
		c.frameCodec = frame.NewRawCodecWithCompression(client.NewBodyCompressor(compression))
		c.segmentCode = segment.NewCodecWithCompression(client.NewPayloadCompressor(compression))
	}
	c.maybeSwitchToModernLayout(outgoing)
	return nil
}

// readWithModernLayout is used to read frame from the connection.
// If the connection is using modern framing layout, it will read segments.
// Otherwise, it will read frames.
func (c *Conn) readWithModernLayout(r io.Reader) (*frame.Frame, error) {
	if c.modernLayoutRead {
		v, err := c.readSegment(r)
		return v, trace.Wrap(err)
	}
	fr, err := c.readFrame(r)
	return fr, trace.Wrap(err)
}

// readFrame is used to read frame from the connection.
// If read frame is a Startup frame, it will switch to modern framing layout and
// update codec to use modern framing layout.
func (c *Conn) readFrame(r io.Reader) (*frame.Frame, error) {
	fr, err := c.frameCodec.DecodeFrame(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if startup, ok := fr.Body.Message.(*message.Startup); ok {
		compression := startup.GetCompression()
		if !compression.IsValid() {
			return nil, trace.BadParameter("invalid compression: %v", compression)
		}
		c.frameCodec = frame.NewRawCodecWithCompression(client.NewBodyCompressor(compression))
		c.segmentCode = segment.NewCodecWithCompression(client.NewPayloadCompressor(compression))
		// If moderate framing layout is supported all received from a client after Startup message should
		// use segment encoding.
		c.modernLayoutRead = fr.Header.Version.SupportsModernFramingLayout()
	}

	return fr, nil
}

func (c *Conn) checkSelfContained() *frame.Frame {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if len(c.selfContained) == 0 {
		return nil
	}
	out := c.selfContained[0]
	c.selfContained = c.selfContained[1:]
	return out
}

// readSegment is used to read segments from the connection.
// If frame is not self-contained, segments are split into multiple frames.
// Read the segments till received bodyLength bytes.
func (c *Conn) readSegment(r io.Reader) (*frame.Frame, error) {
	previousSegment := bytes.Buffer{}
	expectedSegmentSize := 0

	for {
		seg, err := c.segmentCode.DecodeSegment(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if seg.Header.IsSelfContained {
			fr, err := c.readSelfContainedSegment(seg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return fr, nil
		}

		// Otherwise read the frame size and keep reading until we read all data.
		// Segments are always delivered in order.
		if expectedSegmentSize == 0 {
			frameHeader, err := c.frameCodec.DecodeHeader(bytes.NewReader(seg.Payload.UncompressedData))
			if err != nil {
				return nil, trace.Wrap(err)
			}
			expectedSegmentSize = int(primitive.FrameHeaderLengthV3AndHigher + frameHeader.BodyLength)
		}
		// Append another segment
		if _, err := previousSegment.Write(seg.Payload.UncompressedData); err != nil {
			return nil, trace.Wrap(err)
		}
		// Return the frame after reading all segments.
		if expectedSegmentSize == previousSegment.Len() {
			fr, err := c.readFrame(&previousSegment)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return fr, nil
		}
	}
}

func (c *Conn) readSelfContainedSegment(incoming *segment.Segment) (*frame.Frame, error) {
	var out []*frame.Frame
	payloadReader := bytes.NewReader(incoming.Payload.UncompressedData)
	for payloadReader.Len() > 0 {
		fr, err := c.readFrame(payloadReader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, fr)
	}
	if len(out) == 0 {
		return nil, trace.BadParameter("no frames in self-contained segment")
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.selfContained = append(c.selfContained, out[1:]...)
	return out[0], nil
}

// writeFrameWithModernLayout is used to write frame to the connection.
func (c *Conn) writeFrameWithModernLayout(outgoing *frame.Frame) error {
	if c.modernLayoutWrite {
		return trace.Wrap(c.writeSegment(outgoing, c.Conn))
	}
	return trace.Wrap(c.writeFrame(outgoing, c.Conn))
}

// writeFrame is used to write frame to the connection.
func (c *Conn) writeFrame(outgoing *frame.Frame, wr io.Writer) error {
	err := c.frameCodec.EncodeFrame(outgoing, wr)
	return trace.Wrap(err)
}

// writeSegment is used to write segments to the connection.
func (c *Conn) writeSegment(outgoing *frame.Frame, wr io.Writer) error {
	var buff bytes.Buffer
	if err := c.writeFrame(outgoing, &buff); err != nil {
		return trace.Wrap(err)
	}
	seg := &segment.Segment{
		Header:  &segment.Header{IsSelfContained: true},
		Payload: &segment.Payload{UncompressedData: buff.Bytes()},
	}

	if err := c.segmentCode.EncodeSegment(seg, wr); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// maybeSwitchToModernLayout is used to switch to modern framing layout.
// If received frame is a Ready frame or Authenticate frame, it will switch to modern framing layout.
func (c *Conn) maybeSwitchToModernLayout(fr *frame.Frame) {
	if !isReady(fr) && !isAuthenticate(fr) {
		return
	}
	if !c.modernLayoutRead {
		c.modernLayoutRead = fr.Header.Version.SupportsModernFramingLayout()
	}
	if !c.modernLayoutWrite {
		c.modernLayoutWrite = fr.Header.Version.SupportsModernFramingLayout()
	}
}

func isReady(fr *frame.Frame) bool {
	return fr.Header.OpCode == primitive.OpCodeReady
}

func isAuthenticate(fr *frame.Frame) bool {
	return fr.Header.OpCode == primitive.OpCodeAuthenticate
}
