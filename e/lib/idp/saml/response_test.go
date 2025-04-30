package saml

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
)

func TestWriteSAMLPOSTFormWithHeaders(t *testing.T) {
	const testURL = "https://example.com"
	tests := []struct {
		name     string
		formData POSTFormData
	}{
		{
			name: "request form",
			formData: POSTFormData{
				URL:                  testURL,
				SAMLAuthnMessageType: SAMLRequest,
				SAMLAuthnMessage:     "fake_authn_request",
				RelayState:           "fake_relay_state",
			},
		},
		{
			name: "response form",
			formData: POSTFormData{
				URL:                  testURL,
				SAMLAuthnMessageType: SAMLResponse,
				SAMLAuthnMessage:     "fake_authn_response",
				RelayState:           "fake_relay_state",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				err := WriteSAMLPOSTFormWithHeaders(w, test.formData)
				require.NoError(t, err)
			}
			req := httptest.NewRequest("GET", testURL, nil)
			w := httptest.NewRecorder()
			handler(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			// check csp headers
			cspStr := w.Header().Get("Content-Security-Policy")
			csp, err := parseCSP(cspStr)
			require.NoError(t, err)
			valdiateBaseCSPValues(t, csp)
			nonceHexFromScriptDirective := nonceHexValue(csp["script-src"])
			require.NotEmpty(t, nonceHexFromScriptDirective)

			// check form fields
			result := w.Result()
			defer result.Body.Close()
			form, err := html.Parse(result.Body)
			require.NoError(t, err)
			formNode := testenv.FindNode(form, "form")
			require.NotNil(t, formNode)
			var formID string
			switch test.formData.SAMLAuthnMessageType {
			case SAMLRequest:
				formID = requestFormID.String()
			case SAMLResponse:
				formID = responseFormID.String()
			}
			// checks the {{.FormID}}
			require.Equal(t, formID, formNode.Attr[2].Val)

			inputNode := testenv.FindNode(formNode, "input")
			require.Equal(t, test.formData.SAMLAuthnMessageType.String(), inputNode.Attr[1].Val)
			require.Equal(t, test.formData.SAMLAuthnMessage, inputNode.Attr[2].Val)
			require.Equal(t, RelayState.String(), inputNode.NextSibling.NextSibling.Attr[1].Val)
			require.Equal(t, test.formData.RelayState, inputNode.NextSibling.NextSibling.Attr[2].Val)

			// check csp nonce in script tags matches csp directives from the header.
			scriptNode := testenv.FindNode(form, "script")
			require.NotNil(t, scriptNode)
			require.Equal(t, scriptNode.Attr[0].Val, nonceHexFromScriptDirective)
		})
	}
}

func valdiateBaseCSPValues(t *testing.T, csp map[string][]string) {
	require.Equal(t, []string{"'none'"}, csp["object-src"])
	require.Equal(t, []string{"'none'"}, csp["base-uri"])
	require.Equal(t, []string{"'none'"}, csp["frame-ancestors"])
	require.Equal(t, []string{"'none'"}, csp["img-src"])
	require.Equal(t, []string{"'none'"}, csp["style-src"])
}

func nonceHexValue(nonceDirectives []string) string {
	// single submit script nonce is expected.
	nonceStr := nonceDirectives[0]
	return strings.TrimRight(strings.Split(nonceStr, "-")[1], "'")
}

func parseCSP(policyStr string) (map[string][]string, error) {
	policies := strings.Split(policyStr, ";")
	var policyMap = make(map[string][]string)
	for _, policy := range policies {
		parts := strings.Split(strings.TrimSpace(policy), " ")
		if len(parts) == 1 {
			policyMap[parts[0]] = []string{}
			break
		}

		policyMap[parts[0]] = parts[1:]

	}
	return policyMap, nil
}

func TestSetSecurityHeaders(t *testing.T) {
	const submitNonce = "nonce1"
	expectedCspVals := map[string]string{
		"base-uri":        "'none'",
		"script-src":      "'nonce-nonce1'",
		"frame-ancestors": "'none'",
		"object-src":      "'none'",
		"img-src":         "'none'",
		"style-src":       "'none'",
	}

	h := make(http.Header)
	setSecurityHeaders(h, submitNonce)
	actualCsp := h.Get("Content-Security-Policy")
	for k, v := range expectedCspVals {
		expectedCspSubString := fmt.Sprintf("%s %s;", k, v)
		require.Contains(t, actualCsp, expectedCspSubString)
	}

	require.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	require.Equal(t, "strict-origin", h.Get("Referrer-Policy"))
	require.Equal(t, "SAMEORIGIN", h.Get("X-Frame-Options"))
	require.Equal(t, "1; mode=block", h.Get("X-XSS-Protection"))
	require.Equal(t, "max-age=31536000; includeSubDomains", h.Get("Strict-Transport-Security"))
}
