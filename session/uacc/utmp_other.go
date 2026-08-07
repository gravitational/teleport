//go:build !linux

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

package uacc

import (
	"net"
	"time"

	"github.com/gravitational/trace"
)

type UtmpBackend struct{}

func NewUtmpBackend(utmpFile, wtmpFile, btmpFile string) (*UtmpBackend, error) {
	return nil, trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) Login(_, _ string, _ net.Addr, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) Logout(_ string, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) FailedLogin(_ string, _ net.Addr, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) IsUserInFile(_ string, _ string) (bool, error) {
	return false, trace.NotImplemented("utmp is linux only")
}
