/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"net/http"
	"strings"

	"github.com/gravitational/teleport/lib/httplib"
)

func setRedirectPageHeaders(h http.Header) {
	httplib.SetIndexHTMLHeaders(h)
	// Set content policy flags
	var csp = strings.Join([]string{
		// should match the <script> tab nonce (random value)
		"script-src 'nonce-83452726c7f26c'",
		"style-src 'self'",
		"object-src 'self'",
		"img-src 'self'",
	}, ";")

	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", csp)
}

const js = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Teleport Redirection Service</title>
    <script nonce="83452726c7f26c">
      (function() {
        var parts = window.location.hash.split('=');
        if (parts.length === 2 && parts[0] === '#value') {
          const data = { cookie_value: parts[1] };
          fetch('/x-teleport-auth', {
            method: 'POST',
            headers: {
               mode: 'same-origin',
               cache: 'no-store',
               'Content-Type': 'application/json; charset=utf-8',
            },
            body: JSON.stringify(data),
          }).then(() => {
            // redirect to the root and remove current URL from history (back button)
            window.location.replace('/');
          });
        }
      })();
    </script>
  </head>
  <body></body>
</html>
`
