//go:build unix

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

package filesessions

import (
	"encoding/binary"
	"io"
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

func computeVersion(_ string, info os.FileInfo, hash io.Writer) error {
	if err := binary.Write(hash, binary.NativeEndian, info.ModTime().UnixMicro()); err != nil {
		return trace.Wrap(err)
	}
	if err := binary.Write(hash, binary.NativeEndian, info.Size()); err != nil {
		return trace.Wrap(err)
	}
	sys := info.Sys()
	if sys == nil {
		return nil
	}
	sysInfo, ok := sys.(*syscall.Stat_t)
	if !ok {
		return trace.BadParameter("unexpected stat type %T", sys)
	}
	return trace.Wrap(binary.Write(hash, binary.NativeEndian, sysInfo.Ino))
}
