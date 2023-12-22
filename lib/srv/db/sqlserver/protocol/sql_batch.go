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
