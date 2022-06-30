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
	"io"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/gravitational/trace"
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
