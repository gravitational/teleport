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

package dns

import (
	"context"

	"github.com/gravitational/trace"
)

// platformLoadUpstreamNameservers returns an error on Linux. VNet relies
// on systemd-resolved to handle unresolved queries. We should not attempt
// to forward those queries to other upstream servers.
func platformLoadUpstreamNameservers(context.Context) ([]string, error) {
	return nil, trace.NotImplemented("upstream nameserver discovery is not supported on Linux")
}

// Satisfy linter in linux build where withDNSPort isn't referenced.
var _ = withDNSPort
