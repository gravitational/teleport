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
	"io"
	"net/http"
	"slices"
	"strings"

	template "github.com/DataDog/datadog-agent/pkg/template/html"
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
        var currentUrl = new URL(window.location)
        var currentOrigin = currentUrl.origin
        var params = new URLSearchParams(currentUrl.search)
        var stateValue = params.get('state')
        var subjectValue = params.get('subject')
        var path = params.get('path')
        if (!stateValue) {
          return
        }
        // The URL fragment encodes two URLSearchParams values:
        // 'value' is the session cookie, and 'fragment' (optional)
        // is the user's original fragment, reattached to the final
        // navigation below.
        var hashParams = new URLSearchParams(window.location.hash.slice(1))
        var cookieValue = hashParams.get('value')
        if (!cookieValue) {
          return
        }
        var fragment = hashParams.get('fragment')
        const data = {
          state_value: stateValue,
          cookie_value: cookieValue,
          subject_cookie_value: subjectValue,
          required_apps: params.get('required-apps'),
        }
        fetch('/x-teleport-auth', {
          method: 'POST',
          mode: 'same-origin',
          cache: 'no-store',
          headers: {
            'Content-Type': 'application/json; charset=utf-8',
          },
          body: JSON.stringify(data),
        }).then(response => {
          if (!response.ok) {
            return
          }
          var target = currentOrigin
          const nextAppRedirectUrl = response.headers.get("X-Teleport-NextAppRedirectUrl")
          if (nextAppRedirectUrl) {
            // Drop the fragment on a chain hop: reattaching it
            // here would leak it to an intermediate app's
            // origin. The launcher already skips packing it on
            // chain redirects.
            target = nextAppRedirectUrl
          } else {
            try {
              // Resolve path relative to currentOrigin; fall back
              // to the origin root if the path crosses origins
              // (e.g. "//attacker.com/foo").
              var redirectUrl = new URL(path || '/', currentOrigin)
              if (redirectUrl.origin !== currentOrigin) {
                redirectUrl = new URL('/', currentOrigin)
              }
              if (fragment) {
                redirectUrl.hash = fragment
              }
              target = redirectUrl.toString()
            } catch (error) {
              // Malformed URL: target stays as currentOrigin.
            }
          }
          window.location.replace(target)
        })
      })()
    </script>
  </head>
  <body></body>
</html>
`
