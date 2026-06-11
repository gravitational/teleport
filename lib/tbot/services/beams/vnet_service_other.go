//go:build !linux

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

package beams

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

var errVNetUnsupported = &trace.NotImplementedError{
	Message: fmt.Sprintf("service type %q is only supported on linux", VNetServiceType),
}

func platformCreateTUN() (vnet.TUNDevice, error) {
	return nil, errVNetUnsupported
}

func platformUpstreamNameserverSource(*slog.Logger) (dns.UpstreamNameserverSource, error) {
	return nil, errVNetUnsupported
}

func platformConfigureHost(context.Context, vnet.TUNDevice, *vnet.EmbeddedVNetHostConfig) error {
	return errVNetUnsupported
}
