package saml

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"text/template"
	"time"

	"github.com/crewjam/go-xmlsec"
)

// Session represents a user session. It is returned by the
// SessionProvider implementation's GetSession method. Fields here
// are used to set fields in the SAML assertion.
type Session struct {
	ID         string
	CreateTime time.Time
	ExpireTime time.Time
	Index      string

	NameID         string
	Groups         []string
	UserName       string
	UserEmail      string
	UserCommonName string
	UserSurname    string
	UserGivenName  string
}

// SessionProvider is an interface used by IdentityProvider to determine the
// Session associated with a request. For an example implementation, see
// GetSession in the samlidp package.
type SessionProvider interface {
	// GetSession returns the remote user session associated with the http.Request.
	//
	// If (and only if) the request is not associated with a session then GetSession
	// must complete the HTTP request and return nil.
	GetSession(w http.ResponseWriter, r *http.Request, req *IdpAuthnRequest) *Session
}

// IdentityProvider implements the SAML Identity Provider role (IDP).
//
// An identity provider receives SAML assertion requests and responds
// with SAML Assertions.
//
// You must provide a keypair that is used to
// sign assertions.
//
// For each service provider that is able to use this
// IDP you must add their metadata to the ServiceProviders map.
//
// You must provide an implementation of the SessionProvider which
// handles the actual authentication (i.e. prompting for a username
// and password).
type IdentityProvider struct {
	Key              string
	Certificate      string
	MetadataURL      string
	SSOURL           string
	ServiceProviders map[string]*Metadata
	SessionProvider  SessionProvider
}

// Metadata returns the metadata structure for this identity provider.
func (idp *IdentityProvider) Metadata() *Metadata {
	cert, _ := pem.Decode([]byte(idp.Certificate))
	if cert == nil {
		panic("invalid IDP certificate")
	}
	certStr := base64.StdEncoding.EncodeToString(cert.Bytes)

	return &Metadata{
		EntityID:      idp.MetadataURL,
		ValidUntil:    TimeNow().Add(DefaultValidDuration),
		CacheDuration: DefaultValidDuration,
		IDPSSODescriptor: &IDPSSODescriptor{
			ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
			KeyDescriptor: []KeyDescriptor{
				{
					Use: "signing",
					KeyInfo: KeyInfo{
						Certificate: certStr,
					},
				},
				{
					Use: "encryption",
					KeyInfo: KeyInfo{
						Certificate: certStr,
					},
					EncryptionMethods: []EncryptionMethod{
						{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes128-cbc"},
						{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes192-cbc"},
						{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc"},
						{Algorithm: "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"},
					},
				},
			},
			NameIDFormat: []string{
				"urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
			},
			SingleSignOnService: []Endpoint{
				{
					Binding:  HTTPRedirectBinding,
					Location: idp.SSOURL,
				},
				{
					Binding:  HTTPPostBinding,
					Location: idp.SSOURL,
				},
			},
		},
	}
}

// Handler returns an http.Handler that serves the metadata and SSO
// URLs
func (idp *IdentityProvider) Handler() http.Handler {
	mux := http.NewServeMux()

	metadataURL, err := url.Parse(idp.MetadataURL)
	if err != nil {
		panic(err)
	}
	mux.HandleFunc(metadataURL.Path, idp.ServeMetadata)

	ssoURL, err := url.Parse(idp.SSOURL)
	if err != nil {
		panic(err)
	}
	mux.HandleFunc(ssoURL.Path, idp.ServeSSO)
	return mux
}

// ServeMetadata is an http.HandlerFunc that serves the IDP metadata
func (idp *IdentityProvider) ServeMetadata(w http.ResponseWriter, r *http.Request) {
	buf, _ := xml.MarshalIndent(idp.Metadata(), "", "  ")
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	w.Write(buf)
}

// ServeSSO handles SAML auth requests.
//
// When it gets a request for a user that does not have a valid session,
// then it prompts the user via XXX.
//
// If the session already exists, then it produces a SAML assertion and
// returns an HTTP response according to the specified binding. The
// only supported binding right now is the HTTP-POST binding which returns
// an HTML form in the appropriate format with Javascript to automatically
// submit that form the to service provider's Assertion Customer Service
// endpoint.
//
// If the SAML request is invalid or cannot be verified a simple StatusBadRequest
// response is sent.
//
// If the assertion cannot be created or returned, a StatusInternalServerError
// response is sent.
func (idp *IdentityProvider) ServeSSO(w http.ResponseWriter, r *http.Request) {
	req, err := NewIdpAuthnRequest(idp, r)
	if err != nil {
		log.Printf("failed to parse request: %s", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		log.Printf("failed to validate request: %s", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// TODO(ross): we must check that the request ID has not been previously
	//   issued.

	session := idp.SessionProvider.GetSession(w, r, req)
	if session == nil {
		return
	}

	// we have a valid session and must make a SAML assertion
	if err := req.MakeAssertion(session); err != nil {
		log.Printf("failed to make assertion: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := req.WriteResponse(w); err != nil {
		log.Printf("failed to write response: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// ServeIDPInitiated handes an IDP-initiated authorization request. Requests of this
// type require us to know a registered service provider and (optionally) the RelayState
// that will be passed to the application.
func (idp *IdentityProvider) ServeIDPInitiated(w http.ResponseWriter, r *http.Request, serviceProviderID string, relayState string) {
	req := &IdpAuthnRequest{
		IDP:         idp,
		HTTPRequest: r,
		RelayState:  relayState,
	}

	session := idp.SessionProvider.GetSession(w, r, req)
	if session == nil {
		return
	}

	var ok bool
	req.ServiceProviderMetadata, ok = idp.ServiceProviders[serviceProviderID]
	if !ok {
		log.Printf("cannot find service provider: %s", serviceProviderID)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	for _, endpoint := range req.ServiceProviderMetadata.SPSSODescriptor.AssertionConsumerService {
		req.ACSEndpoint = &endpoint
		break
	}

	if err := req.MakeAssertion(session); err != nil {
		log.Printf("failed to make assertion: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := req.WriteResponse(w); err != nil {
		log.Printf("failed to write response: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// IdpAuthnRequest is used by IdentityProvider to handle a single authentication request.
type IdpAuthnRequest struct {
	IDP                     *IdentityProvider
	HTTPRequest             *http.Request
	RelayState              string
	RequestBuffer           []byte
	Request                 AuthnRequest
	ServiceProviderMetadata *Metadata
	ACSEndpoint             *IndexedEndpoint
	Assertion               *Assertion
	AssertionBuffer         []byte
	Response                *Response
}

// NewIdpAuthnRequest returns a new IdpAuthnRequest for the given HTTP request to the authorization
// service.
func NewIdpAuthnRequest(idp *IdentityProvider, r *http.Request) (*IdpAuthnRequest, error) {
	req := &IdpAuthnRequest{
		IDP:         idp,
		HTTPRequest: r,
	}

	switch r.Method {
	case "GET":
		compressedRequest, err := base64.StdEncoding.DecodeString(r.URL.Query().Get("SAMLRequest"))
		if err != nil {
			return nil, fmt.Errorf("cannot decode request: %s", err)
		}
		req.RequestBuffer, err = ioutil.ReadAll(flate.NewReader(bytes.NewReader(compressedRequest)))
		if err != nil {
			return nil, fmt.Errorf("cannot decompress request: %s", err)
		}
		req.RelayState = r.URL.Query().Get("RelayState")
	case "POST":
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		var err error
		req.RequestBuffer, err = base64.StdEncoding.DecodeString(r.PostForm.Get("SAMLRequest"))
		if err != nil {
			return nil, err
		}
		req.RelayState = r.PostForm.Get("RelayState")
	default:
		return nil, fmt.Errorf("method not allowed")
	}
	return req, nil
}

// Validate checks that the authentication request is valid and assigns
// the AuthnRequest and Metadata properties. Returns a non-nil error if the
// request is not valid.
func (req *IdpAuthnRequest) Validate() error {
	if err := xml.Unmarshal(req.RequestBuffer, &req.Request); err != nil {
		return err
	}

	// TODO(ross): is this supposed to be the metdata URL? or the target URL?
	//   i.e. should idp.SSOURL actually be idp.Metadata().EntityID?
	if req.Request.Destination != req.IDP.SSOURL {
		return fmt.Errorf("expected destination to be %q, not %q",
			req.IDP.SSOURL, req.Request.Destination)
	}
	if req.Request.IssueInstant.Add(MaxIssueDelay).Before(TimeNow()) {
		return fmt.Errorf("request expired at %s",
			req.Request.IssueInstant.Add(MaxIssueDelay))
	}
	if req.Request.Version != "2.0" {
		return fmt.Errorf("expected SAML request version 2, got %q", req.Request.Version)
	}

	// find the service provider
	serviceProvider, serviceProviderFound := req.IDP.ServiceProviders[req.Request.Issuer.Value]
	if !serviceProviderFound {
		return fmt.Errorf("cannot handle request from unknown service provider %s",
			req.Request.Issuer.Value)
	}
	req.ServiceProviderMetadata = serviceProvider

	// Check that the ACS URL matches an ACS endpoint in the SP metadata.
	acsValid := false
	for _, acsEndpoint := range serviceProvider.SPSSODescriptor.AssertionConsumerService {
		if req.Request.AssertionConsumerServiceURL == acsEndpoint.Location {
			req.ACSEndpoint = &acsEndpoint
			acsValid = true
			break
		}
	}
	if !acsValid {
		return fmt.Errorf("invalid ACS url specified in request: %s", req.Request.AssertionConsumerServiceURL)
	}

	return nil
}

// MakeAssertion produces a SAML assertion for the
// given request and assigns it to req.Assertion.
func (req *IdpAuthnRequest) MakeAssertion(session *Session) error {
	signatureTemplate := xmlsec.DefaultSignature([]byte(req.IDP.Certificate))
	attributes := []Attribute{}
	if session.UserName != "" {
		attributes = append(attributes, Attribute{
			FriendlyName: "uid",
			Name:         "urn:oid:0.9.2342.19200300.100.1.1",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{{
				Type:  "xs:string",
				Value: session.UserName,
			}},
		})
	}

	if session.UserEmail != "" {
		attributes = append(attributes, Attribute{
			FriendlyName: "eduPersonPrincipalName",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.6",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{{
				Type:  "xs:string",
				Value: session.UserEmail,
			}},
		})
	}
	if session.UserSurname != "" {
		attributes = append(attributes, Attribute{
			FriendlyName: "sn",
			Name:         "urn:oid:2.5.4.4",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{{
				Type:  "xs:string",
				Value: session.UserSurname,
			}},
		})
	}
	if session.UserGivenName != "" {
		attributes = append(attributes, Attribute{
			FriendlyName: "givenName",
			Name:         "urn:oid:2.5.4.42",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{{
				Type:  "xs:string",
				Value: session.UserGivenName,
			}},
		})
	}

	if session.UserCommonName != "" {
		attributes = append(attributes, Attribute{
			FriendlyName: "cn",
			Name:         "urn:oid:2.5.4.3",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []AttributeValue{{
				Type:  "xs:string",
				Value: session.UserCommonName,
			}},
		})
	}

	if len(session.Groups) != 0 {
		groupMemberAttributeValues := []AttributeValue{}
		for _, group := range session.Groups {
			groupMemberAttributeValues = append(groupMemberAttributeValues, AttributeValue{
				Type:  "xs:string",
				Value: group,
			})
		}
		attributes = append(attributes, Attribute{
			FriendlyName: "eduPersonAffiliation",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.1",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values:       groupMemberAttributeValues,
		})
	}

	req.Assertion = &Assertion{
		ID:           fmt.Sprintf("id-%x", randomBytes(20)),
		IssueInstant: TimeNow(),
		Version:      "2.0",
		Issuer: &Issuer{
			Format: "XXX",
			Value:  req.IDP.Metadata().EntityID,
		},
		Signature: &signatureTemplate,
		Subject: &Subject{
			NameID: &NameID{
				Format:          "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
				NameQualifier:   req.IDP.Metadata().EntityID,
				SPNameQualifier: req.ServiceProviderMetadata.EntityID,
				Value:           session.NameID,
			},
			SubjectConfirmation: &SubjectConfirmation{
				Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
				SubjectConfirmationData: SubjectConfirmationData{
					Address:      req.HTTPRequest.RemoteAddr,
					InResponseTo: req.Request.ID,
					NotOnOrAfter: TimeNow().Add(MaxIssueDelay),
					Recipient:    req.ACSEndpoint.Location,
				},
			},
		},
		Conditions: &Conditions{
			NotBefore:    TimeNow(),
			NotOnOrAfter: TimeNow().Add(MaxIssueDelay),
			AudienceRestriction: &AudienceRestriction{
				Audience: &Audience{Value: req.ServiceProviderMetadata.EntityID},
			},
		},
		AuthnStatement: &AuthnStatement{
			AuthnInstant: session.CreateTime,
			SessionIndex: session.Index,
			SubjectLocality: SubjectLocality{
				Address: req.HTTPRequest.RemoteAddr,
			},
			AuthnContext: AuthnContext{
				AuthnContextClassRef: &AuthnContextClassRef{
					Value: "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
				},
			},
		},
		AttributeStatement: &AttributeStatement{
			Attributes: attributes,
		},
	}

	return nil
}

// MarshalAssertion sets `AssertionBuffer` to a signed, encrypted
// version of `Assertion`.
func (req *IdpAuthnRequest) MarshalAssertion() error {
	buf, err := xml.Marshal(req.Assertion)
	if err != nil {
		return err
	}

	buf, err = xmlsec.Sign([]byte(req.IDP.Key),
		buf, xmlsec.SignatureOptions{})
	if err != nil {
		return err
	}

	buf, err = xmlsec.Encrypt(getSPEncryptionCert(req.ServiceProviderMetadata),
		buf, xmlsec.EncryptOptions{})
	if err != nil {
		return err
	}

	req.AssertionBuffer = buf
	return nil
}

// MakeResponse creates and assigns a new SAML response in Response. `Assertion` must
// be non-nill. If MarshalAssertion() has not been called, this function calls it for
// you.
func (req *IdpAuthnRequest) MakeResponse() error {
	if req.AssertionBuffer == nil {
		if err := req.MarshalAssertion(); err != nil {
			return err
		}
	}
	req.Response = &Response{
		Destination:  req.ACSEndpoint.Location,
		ID:           fmt.Sprintf("id-%x", randomBytes(20)),
		InResponseTo: req.Request.ID,
		IssueInstant: TimeNow(),
		Version:      "2.0",
		Issuer: &Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  req.IDP.MetadataURL,
		},
		Status: &Status{
			StatusCode: StatusCode{
				Value: StatusSuccess,
			},
		},
		EncryptedAssertion: &EncryptedAssertion{
			EncryptedData: req.AssertionBuffer,
		},
	}
	return nil
}

// WriteResponse writes the `Response` to the http.ResponseWriter. If
// `Response` is not already set, it calls MakeResponse to produce it.
func (req *IdpAuthnRequest) WriteResponse(w http.ResponseWriter) error {
	if req.Response == nil {
		if err := req.MakeResponse(); err != nil {
			return err
		}
	}
	responseBuf, err := xml.Marshal(req.Response)
	if err != nil {
		return err
	}

	// the only supported binding is the HTTP-POST binding
	switch req.ACSEndpoint.Binding {
	case HTTPPostBinding:
		tmpl := template.Must(template.New("saml-post-form").Parse(`<html>` +
			`<form method="post" action="{{.URL}}" id="SAMLResponseForm">` +
			`<input type="hidden" name="SAMLResponse" value="{{.SAMLResponse}}" />` +
			`<input type="hidden" name="RelayState" value="{{.RelayState}}" />` +
			`<input type="submit" value="Continue" />` +
			`</form>` +
			`<script>document.getElementById('SAMLResponseForm').submit();</script>` +
			`</html>`))
		data := struct {
			URL          string
			SAMLResponse string
			RelayState   string
		}{
			URL:          req.ACSEndpoint.Location,
			SAMLResponse: base64.StdEncoding.EncodeToString(responseBuf),
			RelayState:   req.RelayState,
		}

		buf := bytes.NewBuffer(nil)
		if err := tmpl.Execute(buf, data); err != nil {
			return err
		}
		if _, err := io.Copy(w, buf); err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("%s: unsupported binding %s",
			req.ServiceProviderMetadata.EntityID,
			req.ACSEndpoint.Binding)
	}
}

// getSPEncryptionCert returns the certificate which we can use to encrypt things
// to the SP in PEM format, or nil if no such certificate is found.
func getSPEncryptionCert(sp *Metadata) []byte {
	cert := ""
	for _, keyDescriptor := range sp.SPSSODescriptor.KeyDescriptor {
		if keyDescriptor.Use == "encryption" {
			cert = keyDescriptor.KeyInfo.Certificate
			break
		}
	}

	// If there are no explicitly signing certs, just return the first
	// non-empty cert we find.
	if cert == "" {
		for _, keyDescriptor := range sp.SPSSODescriptor.KeyDescriptor {
			if keyDescriptor.Use == "" && keyDescriptor.KeyInfo.Certificate != "" {
				cert = keyDescriptor.KeyInfo.Certificate
				break
			}
		}
	}

	if cert == "" {
		return nil
	}

	// cleanup whitespace and re-encode a PEM
	cert = regexp.MustCompile("\\s+").ReplaceAllString(cert, "")
	certBytes, _ := base64.StdEncoding.DecodeString(cert)
	certBytes = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes})
	return certBytes
}
