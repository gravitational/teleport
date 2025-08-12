package workloadidentityv1_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestSignX509IssuerCSR(t *testing.T) {
	ctx := t.Context()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.PluginRegistry = plugin.NewRegistry()
			authPlugin, err := eauth.NewPlugin(eauth.Config{License: eauth.ValidLicense{}})
			require.NoError(t, err)
			err = cfg.PluginRegistry.Add(authPlugin)
			require.NoError(t, err)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	clusterName, err := process.GetAuthServer().GetDomainName()
	require.NoError(t, err)

	err = process.GetAuthServer().RotateCertAuthority(ctx, types.RotateRequest{
		Type:        types.SPIFFECA,
		TargetPhase: "init",
		Mode:        "manual",
	})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		const loadSigningKeysFalse = false
		ca, err := process.GetAuthServer().GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.SPIFFECA,
			DomainName: clusterName,
		}, loadSigningKeysFalse)
		require.NoError(t, err)
		require.Len(t, ca.GetActiveKeys().TLS, 1)
		require.Len(t, ca.GetAdditionalTrustedKeys().TLS, 1)
	}, 5*time.Second, 50*time.Millisecond)

	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	const loadSigningKeysFalse = false
	ca, err := client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: clusterName,
	}, loadSigningKeysFalse)
	require.NoError(t, err)

	kps := ca.GetTrustedTLSKeyPairs()
	require.Len(t, kps, 2)

	emptyName, err := asn1.Marshal(new(pkix.Name).ToRDNSequence())
	require.NoError(t, err)
	require.Equal(t, []byte{0x30, 0x00}, emptyName)

	for _, kp := range ca.GetTrustedTLSKeyPairs() {
		caCert, err := tlsca.ParseCertificatePEM(kp.Cert)
		require.NoError(t, err)

		resp, err := client.
			WorkloadIdentityX509OverridesClient().
			SignX509IssuerCSR(ctx, &workloadidentityv1.SignX509IssuerCSRRequest{
				Issuer:          caCert.Raw,
				CsrCreationMode: workloadidentityv1.CSRCreationMode_CSR_CREATION_MODE_SAME,
			})
		require.NoError(t, err)
		csr, err := x509.ParseCertificateRequest(resp.GetCsr())
		require.NoError(t, err)
		require.Equal(t, caCert.RawSubject, csr.RawSubject)
		require.Equal(t, caCert.RawSubjectPublicKeyInfo, csr.RawSubjectPublicKeyInfo)

		resp, err = client.
			WorkloadIdentityX509OverridesClient().
			SignX509IssuerCSR(ctx, &workloadidentityv1.SignX509IssuerCSRRequest{
				Issuer:          caCert.Raw,
				CsrCreationMode: workloadidentityv1.CSRCreationMode_CSR_CREATION_MODE_EMPTY,
			})
		require.NoError(t, err)
		csr, err = x509.ParseCertificateRequest(resp.GetCsr())
		require.NoError(t, err)
		require.Equal(t, emptyName, csr.RawSubject)
		require.Equal(t, caCert.RawSubjectPublicKeyInfo, csr.RawSubjectPublicKeyInfo)
	}
}
