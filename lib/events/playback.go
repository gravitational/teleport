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

package events

import (
	"archive/tar"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// Header returns information about playback
type Header struct {
	// Tar detected tar format
	Tar bool
	// Proto is for proto format
	Proto bool
	// ProtoVersion is a version of the format, valid if Proto is true
	ProtoVersion int64
}

// DetectFormat detects format by reading first bytes
// of the header. Callers should call Seek()
// to reuse reader after calling this function.
func DetectFormat(r io.ReadSeeker) (*Header, error) {
	version := make([]byte, Int64Size)
	_, err := io.ReadFull(r, version)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	protocolVersion := binary.BigEndian.Uint64(version)
	if protocolVersion == ProtoStreamV1 {
		return &Header{
			Proto:        true,
			ProtoVersion: int64(protocolVersion),
		}, nil
	}
	_, err = r.Seek(0, 0)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	tr := tar.NewReader(r)
	_, err = tr.Next()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &Header{Tar: true}, nil
}

// Export converts session files from binary/protobuf to text/JSON.
func Export(ctx context.Context, rs io.ReadSeeker, w io.Writer, exportFormat string) error {
	switch exportFormat {
	case teleport.JSON:
	default:
		return trace.BadParameter("unsupported format %q, %q is the only supported format", exportFormat, teleport.JSON)
	}

	format, err := DetectFormat(rs)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = rs.Seek(0, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	switch {
	case format.Proto:
		protoReader := NewProtoReader(rs)
		for {
			event, err := protoReader.Read(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return trace.Wrap(err)
			}
			switch exportFormat {
			case teleport.JSON:
				data, err := utils.FastMarshal(event)
				if err != nil {
					return trace.ConvertSystemError(err)
				}
				_, err = fmt.Fprintln(w, string(data))
				if err != nil {
					return trace.ConvertSystemError(err)
				}
			default:
				return trace.BadParameter("unsupported format %q, %q is the only supported format", exportFormat, teleport.JSON)
			}
		}
	case format.Tar:
		return trace.BadParameter(
			"to review events in format of Teleport before version 4.4, extract the tarball and look inside")
	default:
		return trace.BadParameter("unsupported format %v", format)
	}
}
