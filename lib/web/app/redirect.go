/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package app

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/net/html"

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

// GetURLFromMetaRedirect parses an HTML redirect response written by
// [MetaRedirect] and returns the redirect URL. Useful for tests.
func GetURLFromMetaRedirect(body io.Reader) (string, error) {
	tokenizer := html.NewTokenizer(body)
	for tt := tokenizer.Next(); tt != html.ErrorToken; tt = tokenizer.Next() {
		token := tokenizer.Token()
		if token.Data != "meta" {
			continue
		}
		if !slices.Contains(token.Attr, html.Attribute{Key: "http-equiv", Val: "refresh"}) {
			continue
		}
		contentAttrIndex := slices.IndexFunc(token.Attr, func(attr html.Attribute) bool { return attr.Key == "content" })
		if contentAttrIndex < 0 {
			return "", trace.BadParameter("refresh tag did not contain content")
		}
		content := token.Attr[contentAttrIndex].Val
		parts := strings.Split(content, "URL=")
		if len(parts) < 2 {
			return "", trace.BadParameter("refresh tag content did not contain URL")
		}
		quotedURL := parts[1]
		return strings.TrimPrefix(strings.TrimSuffix(quotedURL, "'"), "'"), nil
	}
	return "", trace.NotFound("body did not contain refresh tag")
}

var appRedirectTemplate = template.Must(template.New("index").Parse(appRedirectHTML))

const appRedirectHTML = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Teleport Redirection Service</title>
    <script nonce="{{.}}">
      (function() {
        var currentUrl = new URL(window.location);
        var currentOrigin = currentUrl.origin;
        var params = new URLSearchParams(currentUrl.search);
        var stateValue = params.get('state');
        var subjectValue = params.get('subject');
        var path = params.get('path');
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
          required_apps: params.get('required-apps'),
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
            const nextAppRedirectUrl = response.headers.get("X-Teleport-NextAppRedirectUrl")
            if (nextAppRedirectUrl) {
              window.location.replace(nextAppRedirectUrl)
              return;
            }
            try {
              // if a path parameter was passed through the redirect, append that path to the current origin
              if (path) {
                var redirectUrl = new URL(path, currentOrigin)
                if (redirectUrl.origin === currentOrigin) {
                  window.location.replace(redirectUrl.toString())
                } else {
                  window.location.replace(currentOrigin)
                }
              } else {
                window.location.replace(currentOrigin)
              }
            } catch (error) {
              // in case of malformed url, return to current origin
              window.location.replace(currentOrigin)
            }
          }
        });
      })();
    </script>
  </head>
  <body></body>
</html>
`
