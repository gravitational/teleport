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

package authz

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"net/url"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTryParseSVID exercises the URI-SAN-based discrimination used by
// Middleware.GetUser to decide whether a peer cert is an X.509 SPIFFE SVID
// for the local trust domain. The TLS chain verification has already happened
// by the time tryParseSVID runs, so the only checks here are about cert *shape*.
func TestTryParseSVID(t *testing.T) {
	const cluster = "test.cluster.local"
	good := func() *x509.Certificate {
		return &x509.Certificate{
			NotAfter: time.Now().Add(time.Hour),
			URIs:     []*url.URL{spiffeid.RequireFromString("spiffe://" + cluster + "/svc/test").URL()},
		}
	}

	tests := []struct {
		name   string
		cert   *x509.Certificate
		wantOK bool
		wantID string
	}{
		{
			name:   "valid SVID for local trust domain",
			cert:   good(),
			wantOK: true,
			wantID: "spiffe://" + cluster + "/svc/test",
		},
		{
			name: "no URI SAN",
			cert: &x509.Certificate{
				NotAfter: time.Now().Add(time.Hour),
			},
			wantOK: false,
		},
		{
			name: "non-spiffe URI SAN",
			cert: &x509.Certificate{
				NotAfter: time.Now().Add(time.Hour),
				URIs:     []*url.URL{{Scheme: "https", Host: "example.com", Path: "/foo"}},
			},
			wantOK: false,
		},
		{
			name: "multiple spiffe URI SANs (rejected by X509-SVID profile)",
			cert: &x509.Certificate{
				NotAfter: time.Now().Add(time.Hour),
				URIs: []*url.URL{
					spiffeid.RequireFromString("spiffe://" + cluster + "/svc/a").URL(),
					spiffeid.RequireFromString("spiffe://" + cluster + "/svc/b").URL(),
				},
			},
			wantOK: false,
		},
		{
			name: "spiffe URI for a different trust domain",
			cert: &x509.Certificate{
				NotAfter: time.Now().Add(time.Hour),
				URIs:     []*url.URL{spiffeid.RequireFromString("spiffe://other.cluster.local/svc/test").URL()},
			},
			wantOK: false,
		},
		{
			// A realistic Teleport-issued cert has Subject.CommonName (=Username)
			// AND Subject.Organization (=Groups) populated by tlsca.FromSubject.
			// The guard's "Username != \"\" || Groups != \"\"" check kicks the
			// cert out of the SVID branch even if it happens to carry a SPIFFE
			// URI SAN.
			name: "cert with Teleport-identity-shaped subject is not treated as a SVID",
			cert: func() *x509.Certificate {
				c := good()
				c.Subject = pkix.Name{
					CommonName:   "alice",
					Organization: []string{"role-a"},
				}
				return c
			}(),
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := tryParseSVID(tc.cert, cluster)
			if !tc.wantOK {
				assert.False(t, ok, "expected tryParseSVID to reject")
				return
			}
			require.True(t, ok, "expected tryParseSVID to accept")
			require.Equal(t, tc.wantID, id.ID.String())
			require.Equal(t, tc.wantID, id.Identity.Username)
			require.Equal(t, cluster, id.Identity.TeleportCluster)
		})
	}
}
