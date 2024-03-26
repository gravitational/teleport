/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package samlidpconfig

import (
	"net/http"
)

// GCPWorkforcePrams defines input params
// to configure GCP Workforce Identity Federation pool
// and pool provider.
type GCPWorkforcePrams struct {
	// PoolName is a GCP workforce pool name.
	PoolName string
	// PoolProviderName is a GCP workforce pool provider name.
	PoolProviderName string
	// OrganizationID is a GCP organization ID.
	OrganizationID string
	// SAMLIdPMetadataURL is a URL path where Teleport proxy serves
	// the SAML IdP metadata.
	SAMLIdPMetadataURL string
	// HTTPClient is used to fetch metadata from the SAMLIdPMetadataURL
	// endpoint.
	HTTPClient *http.Client
}
