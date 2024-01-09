// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package clusters

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// App describes an app resource.
type App struct {
	// URI is the app URI
	URI uri.ResourceURI

	App types.Application
}

// SAMLIdPServiceProvider describes a SAML IdP resource.
type SAMLIdPServiceProvider struct {
	// URI is the app URI
	URI uri.ResourceURI

	Provider types.SAMLIdPServiceProvider
}
