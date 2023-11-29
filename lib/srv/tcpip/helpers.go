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

package tcpip

import (
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
)

const (
	// maxLengthPrefixedMessageSize is the upper limit of length prefixed messages that we will permit
	// encoding/decoding. Note that this value is actually much larger than required for the usecase that
	// these helpers were written to support.
	maxLengthPrefixedMessageSize = 1024 * 64
)

// WriteLengthPrefixedMessage writes the provided message into the writer with a length prefix to allow
// extraction of the message by ReadLengthPrefixedMessage on the associated reader.
func WriteLengthPrefixedMessage(w io.Writer, msg []byte) error {
	if len(msg) > maxLengthPrefixedMessageSize {
		return trace.Errorf("cannot write %d byte message (exceeds max message size %d)", len(msg), maxLengthPrefixedMessageSize)
	}

	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(len(msg)))

	_, err := w.Write(lb[:])
	if err != nil {
		return trace.Wrap(err)
	}

	if len(msg) == 0 {
		// nothing to write
		return nil
	}

	_, err = w.Write(msg[:])
	return trace.Wrap(err)
}

// ReadLengthPrefixedMessage extracts a length-prefixed message from the reader that was previously
// written into the stream by WriteLengthPrefixedMessage.
func ReadLengthPrefixedMessage(r io.Reader) ([]byte, error) {
	var lb [4]byte
	_, err := io.ReadFull(r, lb[:])
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, trace.Wrap(err)
	}

	msgLen := int(binary.BigEndian.Uint32(lb[:]))
	if msgLen > maxLengthPrefixedMessageSize {
		return nil, trace.Errorf("cannot read %d byte message (exceeds max message size %d)", msgLen, maxLengthPrefixedMessageSize)
	}

	if msgLen == 0 {
		// nothing to read
		return nil, nil
	}

	buf := make([]byte, msgLen)

	_, err = io.ReadFull(r, buf[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return buf, nil
}
