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

// recordingGroupPrefix separates recording group IDs from other digests of a
// certificate, and versions them.
const recordingGroupPrefix = "teleport-app-route-recording-v1"

// newForwardedIdentity returns cert's identity with a route to app, which the
// Proxy forwards with each request to app. It doesn't verify cert, and it
// rejects a user of another cluster.
func newForwardedIdentity(cert *x509.Certificate, clusterName string, app types.Application) (authz.LocalUser, error) {
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
	// The Application Service would reject an identity that fails this.
	if err := appauthz.ValidateForwardedIdentity(user, clusterName); err != nil {
		return authz.LocalUser{}, trace.Wrap(err)
	}
	return user, nil
}

// recordingGroupID derives a session ID from cert and route.
func recordingGroupID(cert *x509.Certificate, route tlsca.RouteToApp) string {
	data := append([]byte(recordingGroupPrefix), cert.Raw...)
	// Length-prefixed, so that different routes can't give the same input.
	for _, field := range []string{route.ClusterName, route.Scope, route.Name, route.PublicAddr} {
		data = binary.AppendUvarint(data, uint64(len(field)))
		data = append(data, field...)
	}
	// Version 8 is RFC 9562's version for implementation-defined contents. The
	// prefix already keeps these digests apart, so the namespace is nil.
	return uuid.NewHash(sha256.New(), uuid.Nil, data, 8).String()
}
