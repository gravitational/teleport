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

package protocol

import (
	"bytes"
	"io"

	"github.com/gravitational/trace"
)

// RefusePacket struct { defines TNS client refuse oracle packet.
type RefusePacket struct {
	*packet
	Message string
}

func parseRefusePacket(bp *packet) (Packet, error) {
	r := bytes.NewReader(bp.buff)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := r.Seek(3, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	message, err := readString(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rp := &RefusePacket{
		packet:  bp,
		Message: message,
	}
	return rp, nil
}
