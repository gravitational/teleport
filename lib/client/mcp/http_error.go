/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package mcp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// httpServerErrorBodyLimit bounds how much of an HTTP 5xx response body is
// kept for the user-facing error message.
const httpServerErrorBodyLimit = 256

// HTTPServerError is returned when a request to the remote MCP server (via
// Teleport) results in an HTTP 5xx response. It preserves the status code,
// the error origin reported by the Teleport Application Service, and a snippet
// of the response body, which are otherwise lost once the response is
// flattened into an error string.
type HTTPServerError struct {
	// StatusCode is the HTTP status code of the response.
	StatusCode int
	// Origin is the value of the Teleport error origin header, empty when the
	// response carries no attribution (e.g. produced by an older Application
	// Service or an intermediate proxy).
	Origin string
	// Body is a bounded, whitespace-collapsed snippet of the response body.
	Body string
}

// Error implements the error interface. The format intentionally includes
// "status <code>" so string-based fallback matching keeps working.
func (e *HTTPServerError) Error() string {
	msg := fmt.Sprintf("request failed with status %d", e.StatusCode)
	if e.Origin != "" {
		msg = fmt.Sprintf("%s (reported by %s)", msg, e.Origin)
	}
	if e.Body != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.Body)
	}
	return msg
}

// AsHTTPServerError returns the HTTPServerError in err's chain, if any.
func AsHTTPServerError(err error) (*HTTPServerError, bool) {
	var httpErr *HTTPServerError
	ok := errors.As(err, &httpErr)
	return httpErr, ok
}

// httpServerErrorRoundTripper converts HTTP 5xx responses into typed
// HTTPServerError errors so callers can attribute the failure instead of
// string-matching a flattened error message.
type httpServerErrorRoundTripper struct {
	base http.RoundTripper
}

func (t *httpServerErrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode < http.StatusInternalServerError {
		return resp, err
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, httpServerErrorBodyLimit))
	resp.Body.Close()
	return nil, trace.Wrap(&HTTPServerError{
		StatusCode: resp.StatusCode,
		Origin:     resp.Header.Get(mcputils.TeleportErrorOriginHeader),
		Body:       strings.Join(strings.Fields(string(body)), " "),
	})
}
