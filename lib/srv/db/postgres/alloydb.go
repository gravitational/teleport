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

package postgres

import (
	"encoding/binary"
	"io"

	"cloud.google.com/go/alloydb/connectors/apiv1beta/connectorspb"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
)

// metadataExchangeAlloyDB performs metadata exchange.
// Based on https://github.com/GoogleCloudPlatform/alloydb-go-connector/blob/1c0174b5d34a97798d3e9d68833e4bf31a4a209c/dialer.go#L491-L579
func metadataExchangeAlloyDB(accessToken string, conn io.ReadWriter) error {
	const alloyDBUserAgent = "teleport/" + teleport.Version

	// send exchange request
	req := &connectorspb.MetadataExchangeRequest{
		UserAgent:   alloyDBUserAgent,
		AuthType:    connectorspb.MetadataExchangeRequest_AUTO_IAM,
		Oauth2Token: accessToken,
	}
	m, err := proto.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}

	var buf []byte
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(m)))
	buf = append(buf, m...)

	_, err = conn.Write(buf)
	if err != nil {
		return trace.Wrap(err)
	}

	// read response
	buf = make([]byte, 4)
	count, err := io.ReadFull(conn, buf)
	if err != nil {
		return trace.Wrap(err)
	}
	if count != len(buf) {
		return trace.BadParameter("expected to read 4 bytes, got %d", count)
	}

	respSize := binary.BigEndian.Uint32(buf)
	const maxLen = 1 << 20 // 1MB
	if respSize > maxLen {
		return trace.BadParameter("response size %d exceeds maximum of %d", respSize, maxLen)
	}

	buf = make([]byte, respSize)
	count, err = io.ReadFull(conn, buf)
	if err != nil {
		return trace.Wrap(err)
	}
	if count != int(respSize) {
		return trace.BadParameter("expected to read %v bytes, got %d", respSize, count)
	}

	// unmarshal
	var mdxResp connectorspb.MetadataExchangeResponse
	err = proto.Unmarshal(buf, &mdxResp)
	if err != nil {
		return trace.Wrap(err)
	}

	// check
	if mdxResp.GetResponseCode() != connectorspb.MetadataExchangeResponse_OK {
		return trace.BadParameter("metadata exchange failed: %v, error: %s", mdxResp.GetResponseCode(), mdxResp.GetError())
	}
	return nil
}
