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

package dns

import "context"

// OSUpstreamNameserverSource provides the list of upstream DNS nameservers
// configured in the OS. The VNet DNS resolver will forward unhandles queries to
// these nameservers.
type OSUpstreamNameserverSource struct{}

// NewOSUpstreamNameserverSource returns a new *OSUpstreamNameserverSource.
func NewOSUpstreamNameserverSource() (*OSUpstreamNameserverSource, error) {
	return &OSUpstreamNameserverSource{}, nil
}

// UpstreamNameservers is net yet implemented and currently returns a nil/empty
// list of upstream nameservers. It does not return an error so that the
// networking stack can actually run without just immediately exiting.
func (s *OSUpstreamNameserverSource) UpstreamNameservers(ctx context.Context) ([]string, error) {
	// TODO(nklaassen): implement UpstreamNameservers on windows.
	return nil, nil
}
