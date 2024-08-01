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

package web

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/gravitational/roundtrip"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetSPIFFEBundle(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	authServer := env.server.Auth()
	cn, err := authServer.GetClusterName()
	require.NoError(t, err)
	ca, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: cn.GetClusterName(),
	}, false)
	require.NoError(t, err)

	var wantCACerts []*x509.Certificate
	for _, certPem := range services.GetTLSCerts(ca) {
		cert, err := tlsca.ParseCertificatePEM(certPem)
		require.NoError(t, err)
		wantCACerts = append(wantCACerts, cert)
	}

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	res, err := clt.Get(ctx, clt.Endpoint("webapi", "spiffe", "bundle.json"), nil)
	require.NoError(t, err)

	td, err := spiffeid.TrustDomainFromString(cn.GetClusterName())
	require.NoError(t, err)
	gotBundle, err := spiffebundle.Read(td, res.Reader())
	require.NoError(t, err)

	require.Len(t, gotBundle.X509Authorities(), len(wantCACerts))
	for _, caCert := range wantCACerts {
		require.True(t, gotBundle.HasX509Authority(caCert), "certificate not found in bundle")
	}
}
