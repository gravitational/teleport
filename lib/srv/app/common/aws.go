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

package common

// AWSResolvedEndpoint is an endpoint that has been resolved based on a
// partition service, and region.
type AWSResolvedEndpoint struct {
	// URL is the base URL endpoint of the service.
	URL string
	// SigningName is service name of the resolved endpoint.
	SigningName string
	// SigningRegion is the region of the resolved endpoint.
	SigningRegion string
}
