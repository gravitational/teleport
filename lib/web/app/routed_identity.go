/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

// recordingGroupPrefix keeps recording group IDs apart from other digests of a
// certificate, and versions how they're derived.
const recordingGroupPrefix = "teleport-app-route-recording-v1"

// newRoutedIdentity returns the identity the Proxy forwards with each request
// it sends to app on behalf of the client certificate cert. It's the
// certificate's identity with an app route added, so every other claim,
// including a scope pin, stays the client's own.
//
// cert must be a client certificate the Proxy verified in the TLS handshake.
// newRoutedIdentity doesn't verify it, but it rejects a user of another cluster,
// as authz.Middleware.GetUser does, since the Proxy also trusts other clusters'
// User CAs.
func newRoutedIdentity(cert *x509.Certificate, clusterName string, app types.Application) (authz.LocalUser, error) {
	identity, err := tlsca.IdentityFromCert(cert)
	if err != nil {
		return authz.LocalUser{}, trace.Wrap(err)
	}
	if identity.TeleportCluster != clusterName {
		return authz.LocalUser{}, trace.AccessDenied("certificate is for a user of cluster %q", identity.TeleportCluster)
	}
	identity.RouteToApp = tlsca.RouteToApp{
		ClusterName: clusterName,
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		Scope:       app.GetScope(),
	}
	identity.RouteToApp.SessionID = recordingGroupID(cert, identity.RouteToApp)
	user := authz.LocalUser{Username: identity.Username, Identity: *identity}
	// The Application Service rejects an identity that fails this, such as one
	// with a Kubernetes route.
	if err := appauthz.ValidateForwardedIdentity(user, clusterName); err != nil {
		return authz.LocalUser{}, trace.Wrap(err)
	}
	return user, nil
}

// recordingGroupID returns the recording group ID of requests to route made
// with cert. It's the same on every Proxy and connection, and a renewed
// certificate starts a new group. The prefix has a fixed length and DER encodes
// its own, so only the route's fields need lengths to keep them apart.
func recordingGroupID(cert *x509.Certificate, route tlsca.RouteToApp) string {
	data := append([]byte(recordingGroupPrefix), cert.Raw...)
	for _, field := range []string{route.ClusterName, route.Scope, route.Name, route.PublicAddr} {
		data = binary.AppendUvarint(data, uint64(len(field)))
		data = append(data, field...)
	}
	// Version 8 is the RFC 9562 version for contents the implementation
	// chooses. The prefix already keeps these digests apart, so the space is nil.
	return uuid.NewHash(sha256.New(), uuid.Nil, data, 8).String()
}
