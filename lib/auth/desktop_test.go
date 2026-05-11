/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth_test

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/winpki"
)

// TestDesktopAccessDisabled makes sure desktop access can be disabled via modules.
// Since desktop connections require a cert, this is mediated via the cert generating function.
func TestDesktopAccessDisabled(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Desktop: {Enabled: false}, // Explicitly turn off desktop access.
			},
		},
	})

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	r, err := p.a.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{})
	require.Nil(t, r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster is not licensed for desktop access, please contact the cluster administrator")
}

func TestDesktopAccessCAOverrides(t *testing.T) {
	t.Parallel()

	tlsServer := newTestTLSServer(t)
	authServer := tlsServer.Auth()
	authServer.SetSubCAEnabled(true)

	cn, err := authServer.GetClusterName(t.Context())
	require.NoError(t, err)
	clusterName := cn.GetClusterName()

	// Fetch the CA and self-signed certificate.
	const loadKeys = false
	ca, err := authServer.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       types.WindowsCA,
		DomainName: clusterName,
	}, loadKeys)
	require.NoError(t, err)
	caCert, err := tlsutils.ParseCertificatePEM(ca.GetActiveKeys().TLS[0].Cert)
	require.NoError(t, err)

	runTest := func(
		t *testing.T,
		wantParent *x509.Certificate,
		wantDetails *proto.CAOverrideCertificateDetails,
	) {
		genResp, err := winpki.GenerateWindowsDesktopCredentials(t.Context(), authServer, &winpki.GenerateCredentialsRequest{
			Username:    "Administrator",
			Domain:      "LLAMA",
			TTL:         1 * time.Hour,
			ClusterName: clusterName,
		})
		require.NoError(t, err, "GenerateWindowsDesktopCredentials errored")

		genCert, err := x509.ParseCertificate(genResp.CertDER)
		require.NoError(t, err)

		assert.NoError(t,
			genCert.CheckSignatureFrom(wantParent),
			"Certificate signature verification failed (self-signed CA)",
		)
		if diff := cmp.Diff(wantDetails, genResp.CAOverrideDetails, protocmp.Transform()); diff != "" {
			t.Errorf("genResp.CAOverrideDetails mismatch (-want +got)\n%s", diff)
		}
	}

	t.Run("generate without override", func(t *testing.T) {
		runTest(t, caCert, nil)
	})

	// Create an active override for caCert.
	externalCA, err := testenv.NewSelfSignedCA(nil)
	require.NoError(t, err)
	env := &testenv.Env{
		Clock:        tlsServer.Clock(),
		ClusterName:  clusterName,
		ExternalRoot: externalCA,
	}
	caOverride := env.NewOverrideForCA(t, ca, nil)
	override := caOverride.Spec.CertificateOverrides[0]
	override.Disabled = false // enable
	_, err = authServer.CreateCertAuthorityOverride(t.Context(), caOverride)
	require.NoError(t, err, "CreateCertAuthorityOverride errored")
	overrideCert, err := tlsutils.ParseCertificatePEM([]byte(override.Certificate))
	require.NoError(t, err)

	t.Run("generate with override", func(t *testing.T) {
		runTest(t, overrideCert, &proto.CAOverrideCertificateDetails{
			PublicKeyHash: override.PublicKey,
		})
	})
}
