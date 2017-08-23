package saml2

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"bytes"
	"compress/flate"

	"github.com/beevik/etree"
	"github.com/russellhaering/goxmldsig"
	require "github.com/stretchr/testify/require"
)

func signResponse(t *testing.T, resp string, sp *SAMLServiceProvider) string {
	doc := etree.NewDocument()
	err := doc.ReadFromBytes([]byte(resp))
	require.NoError(t, err)

	el := doc.Root()

	// Strip existing signatures
	signatures := el.FindElements("//Signature")
	for _, sig := range signatures {
		parent := sig.Parent()
		parent.RemoveChild(sig)
	}

	el, err = sp.SigningContext().SignEnveloped(el)
	require.NoError(t, err)

	doc0 := etree.NewDocument()
	doc0.SetRoot(el)
	doc0.WriteSettings = etree.WriteSettings{
		CanonicalAttrVal: true,
		CanonicalEndTags: true,
		CanonicalText:    true,
	}

	str, err := doc0.WriteToString()
	require.NoError(t, err)
	return str
}

func TestSAML(t *testing.T) {
	block, _ := pem.Decode([]byte(idpCertificate))
	require.NotEmpty(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	require.NotEmpty(t, cert)

	randomKeyStore := dsig.RandomKeyStoreForTest()
	_, _cert, err := randomKeyStore.GetKeyPair()

	cert0, err := x509.ParseCertificate(_cert)
	require.NoError(t, err)
	require.NotEmpty(t, cert0)

	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{cert, cert0},
	}

	sp := &SAMLServiceProvider{
		IdentityProviderSSOURL:      "https://dev-116807.oktapreview.com/app/scaleftdev116807_scaleft_1/exk5zt0r12Edi4rD20h7/sso/saml",
		IdentityProviderIssuer:      "http://www.okta.com/exk5zt0r12Edi4rD20h7",
		AssertionConsumerServiceURL: "http://localhost:8080/v1/_saml_callback",
		SignAuthnRequests:           true,
		AudienceURI:                 "123",
		IDPCertificateStore:         &certStore,
		SPKeyStore:                  randomKeyStore,
		NameIdFormat:                NameIdFormatPersistent,
	}

	authRequestURL, err := sp.BuildAuthURL("/some/link/here")
	require.NoError(t, err)
	require.NotEmpty(t, authRequestURL)

	authRequestString, err := sp.BuildAuthRequest()
	require.NoError(t, err)
	require.NotEmpty(t, authRequestString)

	// Note (Phoebe): The sample responses we acquired expired fairly quickly, meaning that our validation will fail
	// because we check the expiration time;
	// I've modified them to expire in ~100 years and removed their signatures, since those hash values are no longer
	// valid. We have to re-sign them here before validating them
	raw := signResponse(t, rawResponse, sp)

	el, err := sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(raw)))
	require.NoError(t, err)
	require.NotEmpty(t, el)

	assertionInfo, err := sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(raw)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.NotEmpty(t, assertionInfo.WarningInfo)
	require.Equal(t, "phoebe.simon@scaleft.com", assertionInfo.NameID)
	require.Equal(t, "phoebe.simon@scaleft.com", assertionInfo.Values.Get("Email"))
	require.Equal(t, "Phoebe", assertionInfo.Values.Get("FirstName"))
	require.Equal(t, "Simon", assertionInfo.Values.Get("LastName"))
	require.Equal(t, "phoebesimon", assertionInfo.Values.Get("Login"))

	assertionInfoModifiedAudience := signResponse(t, assertionInfoModifiedAudienceResponse, sp)

	assertionInfo, err = sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(assertionInfoModifiedAudience)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.True(t, assertionInfo.WarningInfo.NotInAudience)

	assertionInfoOneTimeUse := signResponse(t, assertionInfoOneTimeUseResponse, sp)

	assertionInfo, err = sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(assertionInfoOneTimeUse)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.True(t, assertionInfo.WarningInfo.OneTimeUse)

	assertionInfoProxyRestriction := signResponse(t, assertionInfoProxyRestrictionResponse, sp)

	assertionInfo, err = sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(assertionInfoProxyRestriction)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.NotEmpty(t, assertionInfo.WarningInfo.ProxyRestriction)
	require.Equal(t, 3, assertionInfo.WarningInfo.ProxyRestriction.Count)
	require.Equal(t, []string{"123"}, assertionInfo.WarningInfo.ProxyRestriction.Audience)

	assertionInfoProxyRestrictionNoCount := signResponse(t, assertionInfoProxyRestrictionNoCountResponse, sp)

	assertionInfo, err = sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(assertionInfoProxyRestrictionNoCount)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.NotEmpty(t, assertionInfo.WarningInfo.ProxyRestriction)
	require.Equal(t, 0, assertionInfo.WarningInfo.ProxyRestriction.Count)
	require.Equal(t, []string{"123"}, assertionInfo.WarningInfo.ProxyRestriction.Audience)

	assertionInfoProxyRestrictionNoAudience := signResponse(t, assertionInfoProxyRestrictionNoAudienceResponse, sp)

	assertionInfo, err = sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(assertionInfoProxyRestrictionNoAudience)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.NotEmpty(t, assertionInfo.WarningInfo.ProxyRestriction)
	require.Equal(t, 3, assertionInfo.WarningInfo.ProxyRestriction.Count)
	require.Equal(t, []string{}, assertionInfo.WarningInfo.ProxyRestriction.Audience)

	assertionInfoResp := signResponse(t, assertionInfoResponse, sp)

	assertionInfo, err = sp.RetrieveAssertionInfo(base64.StdEncoding.EncodeToString([]byte(assertionInfoResp)))
	require.NoError(t, err)
	require.NotEmpty(t, assertionInfo)
	require.NotEmpty(t, assertionInfo.Values)
	require.Equal(t, "phoebe.simon@scaleft.com", assertionInfo.Values.Get("Email"))
	require.Equal(t, "Phoebe", assertionInfo.Values.Get("FirstName"))
	require.Equal(t, "Simon", assertionInfo.Values.Get("LastName"))
	require.Equal(t, "phoebe.simon@scaleft.com", assertionInfo.Values.Get("Login"))

	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(manInTheMiddledResponse)))
	require.Error(t, err)
	require.Equal(t, "Signature could not be verified", err.Error())

	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(alteredReferenceURIResponse)))
	require.Error(t, err)
	// require.IsType(t, ErrInvalidValue{}, err, err.Error())
	require.Equal(t, "Could not verify certificate against trusted certs", err.Error())

	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(alteredSignedInfoResponse)))
	require.Error(t, err)
	require.Equal(t, "Could not verify certificate against trusted certs", err.Error())

	alteredRecipient := signResponse(t, alteredRecipientResponse, sp)
	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(alteredRecipient)))
	require.Error(t, err)
	require.IsType(t, err, ErrInvalidValue{})
	require.Contains(t, err.Error(), "Recipient")

	alteredDestination := signResponse(t, alteredDestinationResponse, sp)
	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(alteredDestination)))
	require.Error(t, err)
	require.IsType(t, err, ErrInvalidValue{})
	require.Equal(t, err.(ErrInvalidValue).Key, "Destination")

	alteredSubjectConfirmationMethod := signResponse(t, alteredSubjectConfirmationMethodResponse, sp)
	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(alteredSubjectConfirmationMethod)))
	require.Error(t, err)
	require.IsType(t, err, ErrInvalidValue{})
	require.Equal(t, err.(ErrInvalidValue).Reason, ReasonUnsupported)
	require.Equal(t, err.(ErrInvalidValue).Key, SubjectConfirmationTag)

	alteredVersion := signResponse(t, alteredVersionResponse, sp)
	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(alteredVersion)))
	require.Error(t, err)
	require.IsType(t, err, ErrInvalidValue{})
	require.Equal(t, err.(ErrInvalidValue).Reason, ReasonUnsupported)
	require.Equal(t, err.(ErrInvalidValue).Key, "SAML version")
	require.Contains(t, err.Error(), "Unsupported SAML version")

	_, err = sp.ValidateEncodedResponse(base64.StdEncoding.EncodeToString([]byte(missingIDResponse)))
	require.Error(t, err)
	require.Equal(t, "Missing ID attribute", err.Error())
}

func TestInvalidResponseBadBase64(t *testing.T) {
	sp := &SAMLServiceProvider{}

	response, err := sp.ValidateEncodedResponse("invalid-base64")
	require.EqualError(t, err, "illegal base64 data at input byte 7")
	require.Nil(t, response)
}

func TestInvalidResponseBadCompression(t *testing.T) {
	sp := &SAMLServiceProvider{}

	// Value from: https://github.com/golang/go/blob/23416315060bf7601e5779c3a6a2529d4d604584/src/compress/flate/flate_test.go#L219
	rawResponse, err := hex.DecodeString("33180700")
	require.NoError(t, err)

	b64Response := base64.StdEncoding.EncodeToString(rawResponse)

	response, err := sp.ValidateEncodedResponse(b64Response)
	require.EqualError(t, err, "flate: corrupt input before offset 3")
	require.Nil(t, response)
}

func TestInvalidResponseBadXML(t *testing.T) {
	sp := &SAMLServiceProvider{}

	compressed := &bytes.Buffer{}

	compressor, err := flate.NewWriter(compressed, flate.BestCompression)
	require.NoError(t, err)

	compressor.Write([]byte(">Definitely&Invalid XML"))
	compressor.Close()

	b64Response := base64.StdEncoding.EncodeToString(compressed.Bytes())

	response, err := sp.ValidateEncodedResponse(b64Response)
	require.EqualError(t, err, "XML syntax error on line 1: invalid character entity &Invalid (no semicolon)")
	require.Nil(t, response)
}

func TestInvalidResponseNoElement(t *testing.T) {
	sp := &SAMLServiceProvider{}

	b64Response := base64.StdEncoding.EncodeToString([]byte("no-element-here"))

	response, err := sp.ValidateEncodedResponse(b64Response)
	require.EqualError(t, err, "unable to parse response")
	require.Nil(t, response)
}
