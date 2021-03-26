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
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/teleport/lib/httplib"
)

func setRedirectPageHeaders(h http.Header, nonce string) {
	httplib.SetIndexHTMLHeaders(h)
	// Set content policy flags
	var csp = strings.Join([]string{
		// Should match the <script> tab nonce (random value).
		fmt.Sprintf("script-src 'nonce-%v'", nonce),
		"style-src 'self'",
		"object-src 'none'",
		"img-src 'self'",
		"base-uri 'self'",
	}, ";")

	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", csp)
}

const js = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Teleport Redirection Service</title>
    <script nonce="%v">
      (function() {
        var searchParts = window.location.search.split('=');
        if (searchParts.length !== 2 || searchParts[0] !== '?state') {
          return;
        }
        var hashParts = window.location.hash.split('=');
        if (hashParts.length !== 2 || hashParts[0] !== '#value') {
          return;
        }
        const data = {
          state_value: searchParts[1],
          cookie_value: hashParts[1],
        };
        fetch('/x-teleport-auth', {
          method: 'POST',
          mode: 'same-origin',
          cache: 'no-store',
          headers: {
            'Content-Type': 'application/json; charset=utf-8',
          },
          body: JSON.stringify(data),
        }).then(response => {
          if (response.ok) {
            // redirect to the root and remove current URL from history (back button)
            window.location.replace('/');
          }
        });
      })();
    </script>
  </head>
  <body></body>
</html>
`
