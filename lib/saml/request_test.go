/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

	body, err := io.ReadAll(w.Result().Body)
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
