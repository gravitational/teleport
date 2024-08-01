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

package spiffe

import (
	"context"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/uds"
)

// Cache acts as an entrypoint for fetching SPIFFE SVIDs.
//
// TODO(noah): Refactor to support multiple attached services.
type Cache struct {
	configuredSVIDs []config.SVIDRequest
}

type Attestation struct {
	UDS *uds.Creds
}

func (c *Cache) GetAll(ctx context.Context, attestation *Attestation) {

}

func (c *Cache) Subscribe() {

}

type Result struct {
	Bundle           *spiffebundle.Bundle
	FederatedBundles map[spiffeid.TrustDomain]*spiffebundle.Bundle
}

func GenerateSVID(ctx context.Context, request config.SVIDRequest) {

}
