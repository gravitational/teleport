/*
Copyright 2015 Gravitational, Inc.

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

// Package httplib implements common utility functions for writing
// classic HTTP handlers
package httplib

import (
	"net/http"
	"strings"
)

// SetNoCacheHeaders tells proxies and browsers do not cache the content
func SetNoCacheHeaders(h http.Header) {
	h.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	h.Set("Pragma", "no-cache")
	h.Set("Expires", "0")
}

// SetStaticFileHeaders sets security header flags for static non-html resources
func SetStaticFileHeaders(h http.Header) {
	SetSameOriginIFrame(h)
	SetNoSniff(h)
}

// SetIndexHTMLHeaders sets security header flags for main index.html page
func SetIndexHTMLHeaders(h http.Header) {
	SetNoCacheHeaders(h)
	SetSameOriginIFrame(h)
	SetNoSniff(h)

	// X-Frame-Options indicates that the page can only be displayed in iframe on the same origin as the page itself
	h.Set("X-Frame-Options", "SAMEORIGIN")

	// X-XSS-Protection is a feature of Internet Explorer, Chrome and Safari that stops pages
	// from loading when they detect reflected cross-site scripting (XSS) attacks.
	h.Set("X-XSS-Protection", "1; mode=block")

	// Once a supported browser receives this header that browser will prevent any communications from
	// being sent over HTTP to the specified domain and will instead send all communications over HTTPS.
	// It also prevents HTTPS click through prompts on browsers
	h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

	// Prevent web browsers from using content sniffing to discover a file’s MIME type
	h.Set("X-Content-Type-Options", "nosniff")

	// Set content policy flags
	var cspValue = strings.Join([]string{
		"script-src 'self'",
		// 'unsafe-inline' needed for reactjs inline styles
		"style-src 'self' 'unsafe-inline'",
		"object-src 'none'",
		"img-src 'self' data: blob:",
	}, ";")

	h.Set("Content-Security-Policy", cspValue)
}

// SetSameOriginIFrame sets X-Frame-Options flag
func SetSameOriginIFrame(h http.Header) {
	// X-Frame-Options indicates that the page can only be displayed in iframe on the same origin as the page itself
	h.Set("X-Frame-Options", "SAMEORIGIN")
}

// SetNoSniff sets X-Content-Type-Options flag
func SetNoSniff(h http.Header) {
	// Prevent web browsers from using content sniffing to discover a file’s MIME type
	h.Set("X-Content-Type-Options", "nosniff")
}

// SetWebConfigHeaders sets headers for webConfig.js
func SetWebConfigHeaders(h http.Header) {
	SetStaticFileHeaders(h)
	h.Set("Content-Type", "application/javascript")
}
