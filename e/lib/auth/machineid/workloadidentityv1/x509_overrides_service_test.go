package workloadidentityv1_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/auth/machineid/workloadidentityv1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func newTestProcess(t *testing.T) (*service.TeleportProcess, *authclient.Client) {
	t.Helper()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.PluginRegistry = plugin.NewRegistry()
			authPlugin, err := eauth.NewPlugin(eauth.Config{
				License:        eauth.ValidLicense{},
				LicenseChecker: eauth.ValidLicense{},
				Modules:        modulestest.OSSModules(),
			})
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

	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	return process, client
}

func TestSignX509IssuerCSR(t *testing.T) {
	ctx := t.Context()

	process, client := newTestProcess(t)

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
			SignX509IssuerCSR(ctx, workloadidentityv1pb.SignX509IssuerCSRRequest_builder{
				Issuer:          caCert.Raw,
				CsrCreationMode: workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_SAME,
			}.Build())
		require.NoError(t, err)
		csr, err := x509.ParseCertificateRequest(resp.GetCsr())
		require.NoError(t, err)
		require.Equal(t, caCert.RawSubject, csr.RawSubject)
		require.Equal(t, caCert.RawSubjectPublicKeyInfo, csr.RawSubjectPublicKeyInfo)

		resp, err = client.
			WorkloadIdentityX509OverridesClient().
			SignX509IssuerCSR(ctx, workloadidentityv1pb.SignX509IssuerCSRRequest_builder{
				Issuer:          caCert.Raw,
				CsrCreationMode: workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_EMPTY,
			}.Build())
		require.NoError(t, err)
		csr, err = x509.ParseCertificateRequest(resp.GetCsr())
		require.NoError(t, err)
		require.Equal(t, emptyName, csr.RawSubject)
		require.Equal(t, caCert.RawSubjectPublicKeyInfo, csr.RawSubjectPublicKeyInfo)
	}
}

func TestX509IssuerOverrideMutationsDeprecated(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	process, authClient := newTestProcess(t)

	overrideClient := authClient.WorkloadIdentityX509OverridesClient()

	const defaultName = "default"

	defaultOverride := workloadidentityv1pb.X509IssuerOverride_builder{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: headerv1pb.Metadata_builder{
			Name: defaultName,
		}.Build(),
	}.Build()

	// Creating a new override is rejected and nothing is persisted.
	//nolint:staticcheck // Deprecated RPC is called on purpose to assert that it is rejected.
	_, err := overrideClient.CreateX509IssuerOverride(ctx, workloadidentityv1pb.CreateX509IssuerOverrideRequest_builder{
		X509IssuerOverride: defaultOverride,
	}.Build())
	require.ErrorIs(t, err, workloadidentityv1.ErrOverrideDeprecated)

	// Upserting is rejected as well, since it can create a new resource.
	//nolint:staticcheck // Deprecated RPC is called on purpose to assert that it is rejected.
	_, err = overrideClient.UpsertX509IssuerOverride(ctx, workloadidentityv1pb.UpsertX509IssuerOverrideRequest_builder{
		X509IssuerOverride: defaultOverride,
	}.Build())
	require.ErrorIs(t, err, workloadidentityv1.ErrOverrideDeprecated)

	_, err = overrideClient.GetX509IssuerOverride(ctx, workloadidentityv1pb.GetX509IssuerOverrideRequest_builder{
		Name: defaultName,
	}.Build())
	require.True(t, trace.IsNotFound(err))

	// Seed the existing resource directly in storage, bypassing the deprecated RPCs.
	_, err = process.GetAuthServer().WorkloadIdentityX509Overrides.CreateX509IssuerOverride(ctx, defaultOverride)
	require.NoError(t, err)

	// Reading the existing override still works.
	got, err := overrideClient.GetX509IssuerOverride(ctx, workloadidentityv1pb.GetX509IssuerOverrideRequest_builder{
		Name: defaultName,
	}.Build())
	require.NoError(t, err)
	require.Equal(t, defaultName, got.GetMetadata().GetName())

	// Updating the existing override is rejected as well.
	//nolint:staticcheck // Deprecated RPC is called on purpose to assert that it is rejected.
	_, err = overrideClient.UpdateX509IssuerOverride(ctx, workloadidentityv1pb.UpdateX509IssuerOverrideRequest_builder{
		X509IssuerOverride: got,
	}.Build())
	require.ErrorIs(t, err, workloadidentityv1.ErrOverrideDeprecated)

	// Listing still works.
	list, err := overrideClient.ListX509IssuerOverrides(ctx, workloadidentityv1pb.ListX509IssuerOverridesRequest_builder{}.Build())
	require.NoError(t, err)
	require.Len(t, list.GetX509IssuerOverrides(), 1)
	require.Equal(t, defaultName, list.GetX509IssuerOverrides()[0].GetMetadata().GetName())

	// Deleting the existing override still works.
	_, err = overrideClient.DeleteX509IssuerOverride(ctx, workloadidentityv1pb.DeleteX509IssuerOverrideRequest_builder{
		Name: defaultName,
	}.Build())
	require.NoError(t, err)

	_, err = overrideClient.GetX509IssuerOverride(ctx, workloadidentityv1pb.GetX509IssuerOverrideRequest_builder{
		Name: defaultName,
	}.Build())
	require.True(t, trace.IsNotFound(err))
}
