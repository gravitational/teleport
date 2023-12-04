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
	"io"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
)

// WriteStreamResponse writes stream response packet to the writer.
func WriteStreamResponse(w io.Writer, tokens []mssql.Token) error {
	var data []byte

	for _, token := range tokens {
		bytes, err := token.Marshal()
		if err != nil {
			return trace.Wrap(err)
		}
		data = append(data, bytes...)
	}

	pkt, err := makePacket(PacketTypeResponse, data)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = w.Write(pkt)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// WriteErrorResponse writes error response to the client.
func WriteErrorResponse(w io.Writer, err error) error {
	return WriteStreamResponse(w, []mssql.Token{
		&mssql.Error{
			Number:  errorNumber,
			Class:   errorClassSecurity,
			Message: err.Error(),
		},
		mssql.DoneToken(),
	})
}
