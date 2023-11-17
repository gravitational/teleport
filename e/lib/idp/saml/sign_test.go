package saml

import (
	"context"
	"crypto/x509"
	"encoding/xml"
	"testing"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestProcessSAMLIdPRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	svcs := samlTestService(ctx, t, clock)

	assertion := &saml.Assertion{
		ID:           "dummy-id",
		IssueInstant: clock.Now(),
		Version:      "2.0",
		Issuer: saml.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  "my-entity-id",
		},
	}

	ssoDescriptorBytes, err := xml.Marshal(saml.SPSSODescriptor{})
	require.NoError(t, err)

	doc := etree.NewDocument()
	doc.SetRoot(assertion.Element())

	assertionBytes, err := doc.WriteToBytes()
	require.NoError(t, err)

	req := &samlidppb.ProcessSAMLIdPRequestRequest{
		Assertion:                    assertionBytes,
		Destination:                  "http://destination",
		RequestId:                    "request-id",
		RequestTime:                  timestamppb.New(assertion.IssueInstant),
		MetadataUrl:                  "https://metadata",
		SignatureMethod:              dsig.RSASHA256SignatureMethod,
		ServiceProviderSsoDescriptor: ssoDescriptorBytes,
	}

	// Admin shouldn't have access
	_, err = svcs.client.ProcessSAMLIdPRequest(withRole(ctx, types.RoleAdmin), req)
	require.True(t, trace.IsAccessDenied(err))

	// Proxy should have access
	resp, err := svcs.client.ProcessSAMLIdPRequest(withRole(ctx, types.RoleProxy), req)
	require.NoError(t, err)

	respDoc := etree.NewDocument()
	require.NoError(t, respDoc.ReadFromBytes(resp.Response))

	cas, err := svcs.client.GetCertAuthorities(ctx, types.SAMLIDPCA, false)
	require.NoError(t, err)
	ca := cas[0]

	caKeySet := ca.GetActiveKeys()
	rawCert := caKeySet.TLS[0].Cert
	require.NotEmpty(t, rawCert)

	cert, err := tlsca.ParseCertificatePEM(rawCert)
	require.NoError(t, err)

	certStore := &dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{
			cert,
		},
	}

	dsigClock := dsig.NewFakeClock(clockwork.NewFakeClockAt(cert.NotBefore))
	validationCtx := dsig.NewDefaultValidationContext(certStore)
	validationCtx.Clock = dsigClock
	_, err = validationCtx.Validate(respDoc.Root())
	require.NoError(t, err)
}
