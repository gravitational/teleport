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

	"github.com/datastax/go-cassandra-native-protocol/frame"
)

// Packet represent cassandra packet frame with
// raw unparsed packet payload.
type Packet struct {
	// raw is raw packet payload.
	raw bytes.Buffer
	// frame is cassandra protocol frame.
	frame *frame.Frame
}

// Frame returns frame.
func (p *Packet) Frame() *frame.Frame {
	return p.frame
}

// FrameBody returns frame body.
func (p *Packet) FrameBody() *frame.Body {
	return p.frame.Body
}

// Header returns frame header.
func (p *Packet) Header() *frame.Header {
	return p.frame.Header
}

// Raw returns raw packet payload.
func (p *Packet) Raw() []byte {
	return p.raw.Bytes()
}
