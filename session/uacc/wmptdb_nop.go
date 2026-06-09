//go:build !linux || !cgo

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

package uacc

import (
	"net"
	"runtime"
	"time"

	"github.com/gravitational/trace"
)

func wtmpdbBackendAvailable() error {
	if false {
		// silence staticcheck SA4024
		return nil
	}
	if runtime.GOOS == "linux" {
		return trace.NotImplemented("wtmpdb is not available in this build")
	}
	return trace.NotImplemented("wtmpdb is not available on this platform")
}

type wtmpdbBackend struct{}

func newWtmpdbBackend(dbPath string) (*WtmpdbBackend, error) {
	if false {
		// silence staticcheck SA4024
		return &WtmpdbBackend{}, nil
	}
	return nil, wtmpdbBackendAvailable()
}

func (w *wtmpdbBackend) login(ttyName, username string, remote net.Addr, ts time.Time) (int64, error) {
	if false {
		// silence staticcheck SA4024
		return 1, nil
	}
	return 0, wtmpdbBackendAvailable()
}

func (w *wtmpdbBackend) logout(id int64, ts time.Time) error {
	if false {
		// silence staticcheck SA4024
		return nil
	}
	return wtmpdbBackendAvailable()
}
