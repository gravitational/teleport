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
	"html/template"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib"
)

const metaRedirectHTML = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<title>Teleport Redirection Service</title>
		<meta http-equiv="cache-control" content="no-cache"/>
		<meta http-equiv="refresh" content="0;URL='{{.}}'" />
	</head>
	<body></body>
</html>
`

var metaRedirectTemplate = template.Must(template.New("meta-redirect").Parse(metaRedirectHTML))

func SetRedirectPageHeaders(h http.Header, nonce string) {
	httplib.SetNoCacheHeaders(h)
	httplib.SetDefaultSecurityHeaders(h)

	// Set content security policy flags
	scriptSrc := "none"
	if nonce != "" {
		// Sets a rule where script can only be ran if the
		// <script> tag contains the same nonce (a random value)
		// we set here.
		scriptSrc = fmt.Sprintf("nonce-%v", nonce)
	}
	httplib.SetRedirectPageContentSecurityPolicy(h, scriptSrc)
}

// MetaRedirect issues a "meta refresh" redirect.
func MetaRedirect(w http.ResponseWriter, redirectURL string) error {
	SetRedirectPageHeaders(w.Header(), "")
	return trace.Wrap(metaRedirectTemplate.Execute(w, redirectURL))
}

var appRedirectTemplate = template.Must(template.New("index").Parse(appRedirectHTML))

const appRedirectHTML = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Teleport Redirection Service</title>
    <script nonce="{{.}}">
      (function() {
        var url = new URL(window.location);
        var params = new URLSearchParams(url.search);
        var searchParts = window.location.search.split('=');
        var stateValue = params.get("state");
        var subjectValue = params.get("subject");
        var path = params.get("path");
        if (!stateValue) {
          return;
        }
        var hashParts = window.location.hash.split('=');
        if (hashParts.length !== 2 || hashParts[0] !== '#value') {
          return;
        }
        const data = {
          state_value: stateValue,
          cookie_value: hashParts[1],
          subject_cookie_value: subjectValue,
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
            try {
              // if a path parameter was passed through the redirect, append that path to the target url
              if (path) {
                var redirectUrl = new URL(path, url.origin)
                window.location.replace(redirectUrl.toString());
              } else {
                window.location.replace(url.origin);
              }
            } catch (error) {
                // in case of malformed url, return to origin
                window.location.replace(url.origin)
            }
          }
        });
      })();
    </script>
  </head>
  <body></body>
</html>
`
