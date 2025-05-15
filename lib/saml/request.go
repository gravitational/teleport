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
	"bytes"
	"html/template"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib"
)

// WriteSAMLPostRequestWithHeaders writes HTML POST form containing SAML
// authentication request.
func WriteSAMLPostRequestWithHeaders(w http.ResponseWriter, rawForm []byte) error {
	setSAMLRequestSecurityHeaders(w.Header())

	htmlBuf := bytes.NewBuffer(nil)
	if err := samlHTTPPostRequest.Execute(htmlBuf, template.HTML(rawForm)); err != nil {
		return trace.Wrap(err)
	}

	if _, err := w.Write(htmlBuf.Bytes()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

var samlHTTPPostRequest = template.Must(template.New("saml-http-post-request").Parse(`
<!doctype html>
<html>
 <head><title>Teleport SAML Service Provider</title></head>
 <body>
  <noscript>
      <p>
        <strong>Note:</strong> Your browser does not support JavaScript,
        you must press the Continue button to proceed.
      </p>
  </noscript>
  {{.}}
  </body>
</html>
`))

// sha256 checksum is calculated for the script tag configured in the form.
// <script>document.getElementById('SAMLSubmitButton').style.visibility="hidden";document.getElementById('SAMLRequestForm').submit();</script>
// The form and script is generated from github.com/russellhaering/gosaml2 library as part of
// http-post binding request generation.
const sha256sum = "'sha256-AjPdJSbZmeWHnEc5ykvJFay8FTWeTeRbs9dutfZ0HqE='"

// TODO(sshah): consolidate security headers used for service provider and identity provider.
func setSAMLRequestSecurityHeaders(h http.Header) {
	h.Set("Content-Security-Policy",
		httplib.GetContentSecurityPolicyString(
			httplib.CSPMap{
				"script-src":      {sha256sum},
				"base-uri":        {"'none'"},
				"frame-ancestors": {"'none'"},
				"object-src":      {"'none'"},
				"img-src":         {"'none'"},
				"style-src":       {"'none'"},
			},
		),
	)

	httplib.SetDefaultSecurityHeaders(h)
}
