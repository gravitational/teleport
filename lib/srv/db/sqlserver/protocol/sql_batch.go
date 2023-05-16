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
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
)

// SQLBatch is a representation of MSServer SQL Batch packet.
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/f2026cd3-9a46-4a3f-9a08-f63140bcbbe3
type SQLBatch struct {
	Packet
	// SQLText contains text batch query.
	SQLText string
}

func toSQLBatch(p Packet) (*SQLBatch, error) {
	if p.Type() != PacketTypeSQLBatch {
		return nil, trace.BadParameter("expected SQLBatch packet, got: %v", p.Type())
	}
	r := bytes.NewReader(p.Data())

	var headersLength uint32
	// The packet header if present only in the first packet.
	if int(p.Header().PacketID) == 1 {
		if err := binary.Read(r, binary.LittleEndian, &headersLength); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if _, err := r.Seek(int64(headersLength), io.SeekStart); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	s, err := mssql.ParseUCS2String(p.Data()[r.Size()-int64(r.Len()):])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SQLBatch{
		Packet:  p,
		SQLText: s,
	}, nil
}
