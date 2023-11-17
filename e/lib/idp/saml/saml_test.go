package saml

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestInitIdP(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	// Standard https port should be stripped off.
	svcs := samlTestServiceWithURL(ctx, t, clock, "https://test.url:443")
	require.Equal(t, "test.url", svcs.samlIdP.metadataURL.Host)

	// A non-standard https port should still be present in the host.
	svcs = samlTestServiceWithURL(ctx, t, clock, "https://test.url:12345")
	require.Equal(t, "test.url:12345", svcs.samlIdP.metadataURL.Host)
}

func TestRotateCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	svcs := samlTestService(ctx, t, clock)

	cas, err := svcs.caService.GetCertAuthorities(ctx, types.SAMLIDPCA, true)
	require.NoError(t, err)
	require.Len(t, cas, 1)
	ca := cas[0]

	// Verify the current certs against the IdP metadata.
	idp, err := svcs.samlIdP.createIdP(ctx)
	require.NoError(t, err)
	certsInIdP(t, ca, &idp)

	// Create a new CA and swap it in.
	newCA := createCA(t)
	require.NoError(t, svcs.caService.CompareAndSwapCertAuthority(newCA, ca))

	// Ensure that the new CA is reflected in the IdP metadata.
	idp, err = svcs.samlIdP.createIdP(ctx)
	require.NoError(t, err)
	certsInIdP(t, newCA, &idp)
}

// certsInIdP ensures that the given cert authority is mirrored in the identity provider.
//
//nolint:revive // Because we want this to be IdP.
func certsInIdP(t *testing.T, ca types.CertAuthority, idp *saml.IdentityProvider) {
	tlsKeys := ca.GetTrustedTLSKeyPairs()
	pemCert := tlsKeys[0].Cert
	cert, err := tlsca.ParseCertificatePEM(pemCert)
	require.NoError(t, err)

	require.Equal(t, cert, idp.Certificate)

	for _, kd := range idp.Metadata().IDPSSODescriptors[0].KeyDescriptors {
		// Massage the pem cert string so that it matches the data from the entity descriptor key
		pemCertString := string(pemCert)
		pemCertString = strings.Replace(pemCertString, "-----BEGIN CERTIFICATE-----", "", 1)
		pemCertString = strings.Replace(pemCertString, "-----END CERTIFICATE-----", "", 1)
		pemCertString = strings.ReplaceAll(pemCertString, "\n", "")

		require.Equal(t, pemCertString, kd.KeyInfo.X509Data.X509Certificates[0].Data)
	}
}
