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

//go:build !(desktop_access_rdp && desktop_encoder)

package rdpclient

import (
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/trace"
)

func EncodeQOIZ(frame []byte, x, y, width, height uint16) ([]*tdpb.FastPathPDU, error) {
	return nil, trace.NotImplemented("qoiz encoding not built into this binary")
}

func EncodeQOIZAvailable() bool { return false }
