package saml2

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/beevik/etree"
	"github.com/satori/go.uuid"
)

const issueInstantFormat = "2006-01-02T15:04:05Z"

func (sp *SAMLServiceProvider) BuildAuthRequestDocument() (*etree.Document, error) {
	authnRequest := &etree.Element{
		Space: "samlp",
		Tag:   "AuthnRequest",
	}

	authnRequest.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	authnRequest.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")

	authnRequest.CreateAttr("ID", "_"+uuid.NewV4().String())
	authnRequest.CreateAttr("Version", "2.0")
	authnRequest.CreateAttr("ProtocolBinding", "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST")
	authnRequest.CreateAttr("AssertionConsumerServiceURL", sp.AssertionConsumerServiceURL)
	authnRequest.CreateAttr("IssueInstant", sp.Clock.Now().UTC().Format(issueInstantFormat))
	authnRequest.CreateAttr("Destination", sp.IdentityProviderSSOURL)

	// NOTE(russell_h): In earlier versions we mistakenly sent the IdentityProviderIssuer
	// in the AuthnRequest. For backwards compatibility we will fall back to that
	// behavior when ServiceProviderIssuer isn't set.
	if sp.ServiceProviderIssuer != "" {
		authnRequest.CreateElement("saml:Issuer").SetText(sp.ServiceProviderIssuer)
	} else {
		authnRequest.CreateElement("saml:Issuer").SetText(sp.IdentityProviderIssuer)
	}

	nameIdPolicy := authnRequest.CreateElement("samlp:NameIDPolicy")
	nameIdPolicy.CreateAttr("AllowCreate", "true")
	nameIdPolicy.CreateAttr("Format", sp.NameIdFormat)

	if sp.RequestedAuthnContext != nil {
		requestedAuthnContext := authnRequest.CreateElement("samlp:RequestedAuthnContext")
		requestedAuthnContext.CreateAttr("Comparison", sp.RequestedAuthnContext.Comparison)

		for _, context := range sp.RequestedAuthnContext.Contexts {
			authnContextClassRef := requestedAuthnContext.CreateElement("saml:AuthnContextClassRef")
			authnContextClassRef.SetText(context)
		}
	}

	doc := etree.NewDocument()

	if sp.SignAuthnRequests {
		signed, err := sp.SignAuthnRequest(authnRequest)
		if err != nil {
			return nil, err
		}

		doc.SetRoot(signed)
	} else {
		doc.SetRoot(authnRequest)
	}
	return doc, nil
}

// SignAuthnRequest takes a document, builds a signature, creates another document
// and inserts the signature in it. According to the schema, the position of the
// signature is right after the Issuer [1] then all other children.
//
// [1] https://docs.oasis-open.org/security/saml/v2.0/saml-schema-protocol-2.0.xsd
func (sp *SAMLServiceProvider) SignAuthnRequest(el *etree.Element) (*etree.Element, error) {
	ctx := sp.SigningContext()

	sig, err := ctx.ConstructSignature(el, true)
	if err != nil {
		return nil, err
	}

	ret := el.Copy()

	var children []etree.Token
	children = append(children, ret.Child[0])     // issuer is always first
	children = append(children, sig)              // next is the signature
	children = append(children, ret.Child[1:]...) // then all other children
	ret.Child = children

	return ret, nil
}

// BuildAuthRequest builds <AuthnRequest> for identity provider
func (sp *SAMLServiceProvider) BuildAuthRequest() (string, error) {
	doc, err := sp.BuildAuthRequestDocument()
	if err != nil {
		return "", err
	}
	return doc.WriteToString()
}

func (sp *SAMLServiceProvider) BuildAuthURLFromDocument(relayState string, doc *etree.Document) (string, error) {
	parsedUrl, err := url.Parse(sp.IdentityProviderSSOURL)
	if err != nil {
		return "", err
	}

	authnRequest, err := doc.WriteToString()
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}

	fw, err := flate.NewWriter(buf, flate.DefaultCompression)
	if err != nil {
		return "", fmt.Errorf("flate NewWriter error: %v", err)
	}

	_, err = fw.Write([]byte(authnRequest))
	if err != nil {
		return "", fmt.Errorf("flate.Writer Write error: %v", err)
	}

	err = fw.Close()
	if err != nil {
		return "", fmt.Errorf("flate.Writer Close error: %v", err)
	}

	qs := parsedUrl.Query()

	qs.Add("SAMLRequest", base64.StdEncoding.EncodeToString(buf.Bytes()))

	if relayState != "" {
		qs.Add("RelayState", relayState)
	}

	parsedUrl.RawQuery = qs.Encode()
	return parsedUrl.String(), nil
}

// BuildAuthURL builds redirect URL to be sent to principal
func (sp *SAMLServiceProvider) BuildAuthURL(relayState string) (string, error) {
	doc, err := sp.BuildAuthRequestDocument()
	if err != nil {
		return "", err
	}
	return sp.BuildAuthURLFromDocument(relayState, doc)
}

// AuthRedirect takes a ResponseWriter and Request from an http interaction and
// redirects to the SAMLServiceProvider's configured IdP, including the
// relayState provided, if any.
func (sp *SAMLServiceProvider) AuthRedirect(w http.ResponseWriter, r *http.Request, relayState string) (err error) {
	url, err := sp.BuildAuthURL(relayState)
	if err != nil {
		return err
	}

	http.Redirect(w, r, url, http.StatusFound)
	return nil
}
