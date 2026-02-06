// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package autoupdate

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	pipePath              = `\\.\pipe\TeleportConnectUpdaterPipe`
	maxUpdateMetadataSize = 1 * 1024 * 1024        // 1 MiB
	maxUpdatePayloadSize  = 1 * 1024 * 1024 * 1024 // 1 GiB
)

type updateMetadata struct {
	// ForceRun determines whether to run the app after installing the update.
	ForceRun bool `json:"force_run"`
	// Version is update version.
	Version string `json:"version"`
}

// writeUpdate writes an update stream in the following order:
//  1. An uint32 specifying the length of the updateMetadata header.
//  2. The updateMetadata header of the specified length.
//  3. The update binary, read until EOF.
func writeUpdate(conn io.Writer, meta updateMetadata, file io.Reader) error {
	if meta.Version == "" {
		return trace.BadParameter("update version is required")
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(metaBytes) > maxUpdateMetadataSize {
		return trace.BadParameter("update metadata payload too large")
	}

	if err = binary.Write(conn, binary.LittleEndian, uint32(len(metaBytes))); err != nil {
		return trace.Wrap(err)
	}
	if _, err = conn.Write(metaBytes); err != nil {
		return trace.Wrap(err)
	}

	_, err = io.Copy(conn, file)
	return trace.Wrap(err)
}

// readUpdate reads an update stream in the following order:
//  1. An uint32 specifying the length of the updateMetadata header.
//  2. The updateMetadata header of the specified length.
//  3. The update binary, read until EOF.
//
// It writes the installer to destinationPath and returns the parsed metadata.
func readUpdate(conn io.Reader, destinationPath string) (*updateMetadata, error) {
	var jsonLen uint32
	if err := binary.Read(conn, binary.LittleEndian, &jsonLen); err != nil {
		return nil, trace.Wrap(err)
	}
	if jsonLen > maxUpdateMetadataSize {
		return nil, trace.BadParameter("update metadata payload too large")
	}

	buf := make([]byte, jsonLen)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	meta := &updateMetadata{}
	if err = json.Unmarshal(buf, meta); err != nil {
		return nil, trace.Wrap(err)
	}
	if meta.Version == "" {
		return nil, trace.BadParameter("update version is required")
	}

	outFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payloadReader := utils.LimitReader(conn, maxUpdatePayloadSize)
	_, err = io.Copy(outFile, payloadReader)
	return meta, trace.NewAggregate(err, outFile.Close())
}
