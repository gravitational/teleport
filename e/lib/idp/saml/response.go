package saml

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/utils"
)

// MessageType defines field types that are used in the SAML IdP redirect URL,
// SAML request or response message in the SAML POST form.
type MessageType string

const (
	// SAMLRequest specifies the SAML authentication request message.
	SAMLRequest MessageType = "SAMLRequest"
	// SAMLResponse specifies the SAML authentication response message.
	SAMLResponse MessageType = "SAMLResponse"
	// RelayState is the SAML authentication request or response relay state.
	RelayState MessageType = "RelayState"
	// Webauthn is the MFA response value that contains the result of
	// Per Session MFA assertion, i.e the credential object created from
	// the result of navigator.credentials.get.
	// TODO(cthach): DELETE IN v20.0.0 WebAuthn field.
	Webauthn MessageType = "Webauthn"
	// MFAResponse is the MFA response value that contains the result of
	// MFA assertion. It can include a WebAuthn or SSO response.
	MFAResponse MessageType = "MFAResponse"
	// SAMLAuthRequest is used to relay base64 encoded original SAML authentication
	// request data when the user is redirected to login page for authentication.
	SAMLAuthRequest = "SAMLAuthRequest"
)

func (m MessageType) String() string {
	return string(m)
}

// FormID identifies SAML request or response form ID.
type FormID string

const (
	// RequestFormID is an ID of SAML request POST form.
	requestFormID FormID = "SAMLRequestForm"
	// ResponseFormID is an ID of SAML response POST form.
	responseFormID FormID = "SAMLResponseForm"
)

func (f FormID) String() string {
	return string(f)
}

// SSORedirectURL returns redirect URL as per the incoming SAML authentication
// request protocol binding: HTTP-POST (POST method) and HTTP-Redirect (GET method)
// protocol binding. For the HTTP-Redirect binding, we can construct the redirect URL
// by copying original URL query params. But for the HTTP-POST binding, we need to
// parse the form values and append a new query named "Method" so that the withSAMLAuth
// middleware can convert the GET request to POST after user is redirected back to
// the IdP after authentication. In both the cases, the query param is base64 encoded
// and placed under SAMLAuthRequest query param so that original request survives
// multiple redirects to and from the login page..
func SSORedirectURL(r *http.Request, redirectPath string) (*url.URL, error) {
	// Authentication message available in URL query for HTTP-Redirect binding request.
	redirectQuery := ""
	if r.URL.RawQuery != "" {
		redirectQuery = url.Values{
			SAMLAuthRequest: []string{base64.URLEncoding.EncodeToString([]byte(r.URL.RawQuery))},
		}.Encode()
	}

	if r.Method == http.MethodPost {
		// In an HTTP-POST binding request, the authentication message is sent
		// in a POST form.
		if err := r.ParseForm(); err != nil {
			return nil, trace.Wrap(err)
		}
		queryString := url.Values{
			SAMLRequest.String(): []string{r.Form.Get(SAMLRequest.String())},
			RelayState.String():  []string{r.Form.Get(RelayState.String())},
			"Method":             []string{http.MethodPost},
		}.Encode()
		redirectQuery = url.QueryEscape(url.Values{
			SAMLAuthRequest: []string{base64.URLEncoding.EncodeToString([]byte(queryString))},
		}.Encode())
	}

	return &url.URL{
		Scheme:   "https",
		Host:     r.Host,
		Path:     redirectPath,
		RawQuery: redirectQuery,
	}, nil
}

// POSTFormData provides form data for postFormTemplate.
type POSTFormData struct {
	// FormID is an HTML POST form ID.
	FormID FormID
	// URL is used to provide URL value for form action. For SAML response,
	// the value of the URL should be an ACS URL. For SAML request, the value
	// of the URL should be full URL of SAML IdP SP-initiated SSO endpoint.
	URL string
	// SubmitScriptNonce is a 128 bit nonce value for the javascript that
	// submits the form.
	SubmitScriptNonce string
	// SAMLAuthnMessageType specifies either SAMLRequest or SAMLResponse type.
	SAMLAuthnMessageType MessageType
	// SAMLAuthnMessage specifies value for the SAMLAuthnMessageType.
	SAMLAuthnMessage string
	// RelayState is SAML authentication request or response relay state.
	// RelayState is an optional field.
	RelayState string
}

func (formData *POSTFormData) checkAndSetDefaults() error {
	if formData.URL == "" {
		return trace.BadParameter("URL cannot be empty")
	}
	if formData.SAMLAuthnMessageType == "" {
		return trace.BadParameter("SAMLAuthnMessageType cannot be empty")
	}
	if formData.SAMLAuthnMessage == "" {
		return trace.BadParameter("SAMLAuthnMessage cannot be empty")
	}
	if formData.SubmitScriptNonce == "" {
		return trace.BadParameter("SubmitScriptNonce cannot be empty")
	}
	if formData.FormID == "" {
		if formData.SAMLAuthnMessageType == SAMLRequest {
			formData.FormID = requestFormID
		}
		if formData.SAMLAuthnMessageType == SAMLResponse {
			formData.FormID = responseFormID
		}
	}

	return nil
}

// WriteSAMLPOSTFormWithHeaders writes an auto-submit HTML POST form with security headers.
// The form can contain either SAML authentication request or response form.
func WriteSAMLPOSTFormWithHeaders(w http.ResponseWriter, formData POSTFormData) error {
	submitScriptNonce, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	formData.SubmitScriptNonce = submitScriptNonce
	if err := formData.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	setSecurityHeaders(w.Header(), submitScriptNonce)

	formBuf := bytes.NewBuffer(nil)
	if err := postFormTemplate.Execute(formBuf, formData); err != nil {
		return trace.Wrap(err)
	}

	if _, err := w.Write(formBuf.Bytes()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// postFormTemplate is an HTML POST form template that can be used for both SAML
// authentication request or response. The script tag auto submits the form.
// Note: It isn't generally expected for this to be run in a browser where javascript is
// disabled. But should we ever have a login form that works without javascript,
// this form will work fine in that scenario.
var postFormTemplate = template.Must(template.New("saml-post-form").Parse(`
<!doctype html>
<html>
 <head><title>Teleport SAML IdP</title></head>
 <body>
  <noscript>
      <p>
        <strong>Note:</strong> Your browser does not support JavaScript,
        you must press the Continue button to proceed.
      </p>
  </noscript>
  <form method="post" action="{{.URL}}" id="{{.FormID}}">
    <input
      type="hidden"
      name="{{.SAMLAuthnMessageType}}"
      value="{{.SAMLAuthnMessage}}"
    />
    <input type="hidden" name="RelayState" value="{{.RelayState}}" />
	<noscript> <input id="SAMLSubmitButton" type="submit" value="Continue" /> </noscript>
  </form>
  <script nonce="{{.SubmitScriptNonce}}">document.getElementById("{{.FormID}}").submit();</script>
  </body>
</html>
`))

// setSecurityHeaders sets CSP headers and security headers
// applied from httplib.SetDefaultSecurityHeaders.
func setSecurityHeaders(h http.Header, submitScriptNonce string) {
	h.Set("Content-Security-Policy",
		httplib.GetContentSecurityPolicyString(
			samlIdPCSP(submitScriptNonce),
		),
	)

	httplib.SetDefaultSecurityHeaders(h)
}

// samlIdPCSP returns Content Security Policy for SAML POST form response.
// samlIdPCSP satisfies strict CSP structure - https://web.dev/articles/strict-csp#structure.
// The only script that is allowed is the one that auto submit's the SAML POST form.
//
// Note: The Web API server sets it's own CSP header for "/web" path. But here
// we are using a custom CSP header for the IDP "/enterprise/saml-idp/*" because it
// allow us to scope down permitted script src whereas the base CSP
// header for the Web UI needs to be more permissive due to its functional requirements.
func samlIdPCSP(submitNonce string) httplib.CSPMap {
	return httplib.CSPMap{
		"script-src": {
			fmt.Sprintf("'nonce-%s'", submitNonce),
		},
		"base-uri":        {"'none'"},
		"frame-ancestors": {"'none'"},
		"object-src":      {"'none'"},
		"img-src":         {"'none'"},
		"style-src":       {"'none'"}}
}
