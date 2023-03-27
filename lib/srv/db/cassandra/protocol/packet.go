/*
Copyright 2022 Gravitational, Inc.

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
