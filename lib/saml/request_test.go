package saml

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteSAMLPostRequestWithHeaders(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		WriteSAMLPostRequestWithHeaders(w, []byte(postform))
	}

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Teleport SAML Service Provider")

	cspStr := w.Header().Get("Content-Security-Policy")
	csp, err := parseCSP(cspStr)
	require.NoError(t, err)
	valdiateBaseCSPValues(t, csp)
	require.Equal(t, sha256sum, csp["script-src"][0])
}

const postform = `
 <form method="POST" action="https://example.com" 
 id="SAMLRequestForm"><input type="hidden" name="SAMLRequest" 
 value="PHNhbY1ZXN0Pg==" />
 <input id="SAMLSubmitButton" type="submit" value="Submit" />
 </form>
 <script>document.getElementById('SAMLSubmitButton').style.visibility="hidden";
 document.getElementById('SAMLRequestForm').submit();</script>
`

func valdiateBaseCSPValues(t *testing.T, csp map[string][]string) {
	require.Equal(t, []string{"'none'"}, csp["object-src"])
	require.Equal(t, []string{"'none'"}, csp["base-uri"])
	require.Equal(t, []string{"'none'"}, csp["frame-ancestors"])
	require.Equal(t, []string{"'none'"}, csp["img-src"])
	require.Equal(t, []string{"'none'"}, csp["style-src"])
}

// TODO(sshah): share csp parser between service provider and identity provider.
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
